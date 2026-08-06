// Package main starts the Canterbury MCP server.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/grpchealth"
	"github.com/cthierer/canterbury/gen/go/canterbury/vault/v1/vaultv1connect"
	"github.com/cthierer/canterbury/internal/adapters/healthgrpc"
	"github.com/cthierer/canterbury/internal/adapters/healthhttp"
	"github.com/cthierer/canterbury/internal/adapters/healthstatic"
	"github.com/cthierer/canterbury/internal/app/health"
	"github.com/cthierer/canterbury/internal/cliapp"
	"github.com/cthierer/canterbury/internal/interfaces/healthcli"
	healthhandler "github.com/cthierer/canterbury/internal/interfaces/healthhttp"
	"github.com/cthierer/canterbury/internal/interfaces/mcphttp"
	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

const (
	serverName           = "canterbury-vault"
	readHeaderTimeout    = 5 * time.Second
	idleTimeout          = 60 * time.Second
	shutdownGracePeriod  = 10 * time.Second
	mcpPath              = "/mcp"
	healthPath           = "/health"
	defaultMCPServerAddr = "127.0.0.1:50053"
	defaultHealthTimeout = 2 * time.Second
)

var (
	buildVersion  = "dev"
	buildRevision = "unknown"
)

type serveConfig struct {
	Addr  string
	Vault struct {
		BaseURL        string        `default:"http://127.0.0.1:50051" split_words:"true"`
		RequestTimeout time.Duration `default:"10s" split_words:"true"`
	}
}

type healthcheckConfig struct {
	URL     string        `envconfig:"MCP_SERVER_HEALTHCHECK_URL"`
	Timeout time.Duration `envconfig:"MCP_SERVER_HEALTHCHECK_TIMEOUT"`
}

func main() {
	ctx := context.Background()
	exitCode := run(ctx, os.Args[1:], os.Stdout)
	os.Exit(exitCode)
}

func run(ctx context.Context, args []string, output io.Writer) int {
	app := cliapp.Application{
		Name:           "mcp-server",
		DefaultCommand: "serve",
		Prepare:        loadLocalEnv,
		Commands: []cliapp.Command{
			{Name: "serve", Summary: "Start the MCP service", Run: runServeCommand},
			{Name: "healthcheck", Summary: "Determine if an MCP service is healthy", Run: runHealthcheckCommand},
		},
		Footer: `Run "mcp-server healthcheck --help" for healthcheck flags.`,
	}

	return app.Run(ctx, args, output)
}

func runServeCommand(ctx context.Context, args []string, _ io.Writer) error {
	cfg, err := loadServeConfig(args)
	if err != nil {
		return fmt.Errorf("load MCP server configuration: %w", err)
	}

	if err := serve(ctx, cfg); err != nil {
		return fmt.Errorf("MCP server stopped: %w", err)
	}

	return nil
}

func runHealthcheckCommand(ctx context.Context, args []string, output io.Writer) error {
	cfg, err := loadHealthcheckConfig(args, output)
	if err != nil {
		return fmt.Errorf("load healthcheck configuration: %w", err)
	}

	return healthcheck(ctx, cfg)
}

func serve(ctx context.Context, cfg serveConfig) error {
	var stop context.CancelFunc
	ctx, stop = signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	httpClient := &http.Client{Timeout: cfg.Vault.RequestTimeout}
	vaultClient := vaultv1connect.NewVaultServiceClient(
		httpClient,
		cfg.Vault.BaseURL,
		connect.WithInterceptors(mcphttp.NewForwardMetadataInterceptor("canterbury-mcp-server/"+buildVersion)),
	)
	mcpHandler := mcphttp.NewHandler(vaultClient, serverName, buildVersion)

	vaultHealth := grpchealth.NewClient(
		httpClient,
		cfg.Vault.BaseURL,
	)

	vaultHealthChecker, err := healthgrpc.NewChecker(vaultHealth, vaultv1connect.VaultServiceName)
	if err != nil {
		return fmt.Errorf("initialize vault health checker: %w", err)
	}

	mcpHealthChecker := healthstatic.NewChecker()
	liveService, err := health.NewService(mcpHealthChecker)
	if err != nil {
		return fmt.Errorf("initialize health service for live checks: %w", err)
	}

	readyService, err := health.NewService(mcpHealthChecker, vaultHealthChecker)
	if err != nil {
		return fmt.Errorf("initialize health service for ready checks: %w", err)
	}

	healthHandler, err := healthhandler.NewHandler(liveService, readyService)
	if err != nil {
		return fmt.Errorf("initialize health handler: %w", err)
	}

	handler := http.NewServeMux()
	handler.Handle(healthPath+"/", http.StripPrefix(healthPath, healthHandler))
	handler.Handle(mcpPath, mcpHandler)

	requestsCtx, cancelRequests := context.WithCancel(context.Background())
	defer cancelRequests()
	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
		BaseContext: func(net.Listener) context.Context {
			return requestsCtx
		},
	}

	errs := make(chan error, 1)
	go func() {
		slog.InfoContext(
			ctx,
			"starting MCP server",
			"address", cfg.Addr,
			"path", mcpPath,
			"version", buildVersion,
			"revision", buildRevision,
		)
		errs <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
		defer cancel()

		mcpHealthChecker.SetNotServing()
		if err := server.Shutdown(shutdownCtx); err != nil {
			cancelRequests()
			if closeErr := server.Close(); closeErr != nil {
				err = errors.Join(err, closeErr)
			}
			return fmt.Errorf("shut down MCP server: %w", err)
		}
		cancelRequests()
		return nil
	case err := <-errs:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve MCP HTTP: %w", err)
	}
}

func healthcheck(ctx context.Context, cfg healthcheckConfig) error {
	healthURL, err := url.Parse(cfg.URL)
	if err != nil {
		return fmt.Errorf("parse health URL: %w", err)
	}

	client := &http.Client{Timeout: cfg.Timeout}
	mcpHealthChecker, err := healthhttp.NewChecker(healthURL, client)
	if err != nil {
		return fmt.Errorf("initialize mcp health checker: %w", err)
	}

	healthApp, err := health.NewService(mcpHealthChecker)
	if err != nil {
		return fmt.Errorf("initialize health application: %w", err)
	}

	healthService, err := healthcli.NewService(healthApp)
	if err != nil {
		return fmt.Errorf("initialize health service: %w", err)
	}

	return healthService.Check(ctx, cfg.Timeout)
}

func loadServeConfig(args []string) (serveConfig, error) {
	if len(args) > 0 {
		return serveConfig{}, fmt.Errorf("unexpected serve argument %q", args[0])
	}

	cfg := serveConfig{Addr: defaultMCPServerAddr}
	if err := envconfig.Process("mcp_server", &cfg); err != nil {
		return serveConfig{}, err
	}
	if err := validateServeConfig(&cfg); err != nil {
		return serveConfig{}, err
	}

	return cfg, nil
}

func loadHealthcheckConfig(args []string, output io.Writer) (healthcheckConfig, error) {
	address := struct{ Addr string }{Addr: defaultMCPServerAddr}
	if err := envconfig.Process("mcp_server", &address); err != nil {
		return healthcheckConfig{}, err
	}

	addr := strings.TrimSpace(address.Addr)
	if addr == "" {
		return healthcheckConfig{}, fmt.Errorf("MCP server address must not be empty")
	}

	cfg := healthcheckConfig{
		URL:     "http://" + addr + healthPath + "/live",
		Timeout: defaultHealthTimeout,
	}
	defaultURL := cfg.URL
	if err := envconfig.Process("mcp_server", &cfg); err != nil {
		return healthcheckConfig{}, err
	}
	if strings.TrimSpace(cfg.URL) == "" {
		cfg.URL = defaultURL
	}

	parsed, err := healthcli.ParseConfig(
		args,
		output,
		healthcli.Config{URL: cfg.URL, Timeout: cfg.Timeout},
		healthcli.ConfigOptions{
			CommandName:  "mcp-server healthcheck",
			URLUsage:     "health endpoint URL",
			NormalizeURL: normalizeHealthURL,
		},
	)
	if err != nil {
		return healthcheckConfig{}, err
	}
	cfg.URL = parsed.URL
	cfg.Timeout = parsed.Timeout

	return cfg, nil
}

func validateServeConfig(config *serveConfig) error {
	config.Addr = strings.TrimSpace(config.Addr)
	if config.Addr == "" {
		return fmt.Errorf("MCP server address must not be empty")
	}

	baseURL, err := normalizeBaseURL(config.Vault.BaseURL)
	if err != nil {
		return fmt.Errorf("validate vault base URL: %w", err)
	}
	config.Vault.BaseURL = baseURL

	if config.Vault.RequestTimeout <= 0 {
		return fmt.Errorf("vault request timeout must be positive")
	}

	return nil
}

func normalizeBaseURL(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", err
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("scheme must be http or https")
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("host is required")
	}
	if parsed.User != nil {
		return "", fmt.Errorf("user information is not allowed")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("query and fragment are not allowed")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", fmt.Errorf("path must be empty")
	}

	parsed.Path = ""
	return parsed.String(), nil
}

func normalizeHealthURL(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", err
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("scheme must be http or https")
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("host is required")
	}
	if parsed.User != nil {
		return "", fmt.Errorf("user information is not allowed")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("query and fragment are not allowed")
	}

	return parsed.String(), nil
}

func loadLocalEnv() error {
	if err := godotenv.Load(".env"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("load dotenv configuration: %w", err)
	}

	return nil
}
