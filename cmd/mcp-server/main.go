// Package main starts the Canterbury MCP server.
package main

import (
	"context"
	"errors"
	"flag"
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
	"github.com/cthierer/canterbury/internal/interfaces/healthcli"
	healthhandler "github.com/cthierer/canterbury/internal/interfaces/healthhttp"
	"github.com/cthierer/canterbury/internal/interfaces/mcphttp"
	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

const (
	serverName           = "canterbury-vault"
	serverVersion        = "dev"
	readHeaderTimeout    = 5 * time.Second
	idleTimeout          = 60 * time.Second
	shutdownGracePeriod  = 10 * time.Second
	serverUserAgent      = "canterbury-mcp-server/" + serverVersion
	mcpPath              = "/mcp"
	healthPath           = "/health"
	defaultMCPServerAddr = "127.0.0.1:50053"
	defaultHealthTimeout = 2 * time.Second
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
	cmd, err := parseCommand(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			writeUsage(output)
			return 0
		}

		slog.ErrorContext(ctx, "parse CLI command", "err", err)
		return 1
	}

	if err := loadLocalEnv(); err != nil {
		slog.ErrorContext(ctx, "load local environment", "err", err)
		return 1
	}

	switch cmd {
	case commandServe:
		cfg, err := loadServeConfig(args[1:])
		if err != nil {
			slog.ErrorContext(ctx, "load MCP server configuration", "err", err)
			return 1
		}

		if err := serve(ctx, cfg); err != nil {
			slog.ErrorContext(ctx, "MCP server stopped", "err", err)
			return 1
		}
	case commandHealthcheck:
		cfg, err := loadHealthcheckConfig(args[1:], output)
		if err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return 0
			}

			slog.ErrorContext(ctx, "load healthcheck configuration", "err", err)
			return 1
		}

		if err := healthcheck(ctx, cfg); err != nil {
			slog.ErrorContext(ctx, "Healthcheck errored", "err", err)
			return 1
		}
	default:
		slog.ErrorContext(ctx, "Unrecognized command", "command", cmd)
		return 1
	}

	return 0
}

func serve(ctx context.Context, cfg serveConfig) error {
	var stop context.CancelFunc
	ctx, stop = signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	httpClient := &http.Client{Timeout: cfg.Vault.RequestTimeout}
	vaultClient := vaultv1connect.NewVaultServiceClient(
		httpClient,
		cfg.Vault.BaseURL,
		connect.WithInterceptors(mcphttp.NewForwardMetadataInterceptor(serverUserAgent)),
	)
	mcpHandler := mcphttp.NewHandler(vaultClient, serverName, serverVersion)

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
		slog.InfoContext(ctx, "starting MCP server", "address", cfg.Addr, "path", mcpPath)
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

var (
	errHealthcheckFailed = errors.New("healthcheck failed")
)

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

	healthcheckCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	if !healthService.Serving(healthcheckCtx) {
		return errHealthcheckFailed
	}

	return nil
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
	if err := envconfig.Process("mcp_server", &cfg); err != nil {
		return healthcheckConfig{}, err
	}

	flags := flag.NewFlagSet("mcp-server healthcheck", flag.ContinueOnError)
	flags.SetOutput(output)
	flags.StringVar(&cfg.URL, "url", cfg.URL, "health endpoint URL")
	flags.DurationVar(&cfg.Timeout, "timeout", cfg.Timeout, "healthcheck timeout")
	flags.Usage = func() {
		writeHealthcheckUsage(output, flags)
	}
	if err := flags.Parse(args); err != nil {
		return healthcheckConfig{}, err
	}
	if flags.NArg() > 0 {
		return healthcheckConfig{}, fmt.Errorf("unexpected healthcheck argument %q", flags.Arg(0))
	}
	if err := validateHealthcheckConfig(&cfg); err != nil {
		return healthcheckConfig{}, err
	}

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

func validateHealthcheckConfig(config *healthcheckConfig) error {
	healthURL, err := normalizeHealthURL(config.URL)
	if err != nil {
		return fmt.Errorf("validate health URL: %w", err)
	}
	config.URL = healthURL

	if config.Timeout <= 0 {
		return fmt.Errorf("healthcheck timeout must be positive")
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

type command string

const (
	commandHealthcheck command = "healthcheck"
	commandServe       command = "serve"
)

var (
	errUnknownCommand = errors.New("unknown command")
)

func parseCommand(args []string) (command, error) {
	if len(args) < 1 {
		return commandServe, nil
	}

	commandString := strings.TrimSpace(args[0])

	switch strings.ToLower(commandString) {
	case "-h", "--help", "help":
		return "", flag.ErrHelp
	case "healthcheck":
		return commandHealthcheck, nil
	case "serve":
		return commandServe, nil
	}

	return "", fmt.Errorf("parsing command: %q: %w", commandString, errUnknownCommand)
}

func writeUsage(output io.Writer) {
	_, _ = fmt.Fprint(output, `Usage:
	mcp-server [command]

Commands:
	serve        Start the MCP service
	healthcheck  Determine if an MCP service is healthy

Run "mcp-server healthcheck --help" for healthcheck flags.
`)
}

func writeHealthcheckUsage(output io.Writer, flags *flag.FlagSet) {
	_, _ = fmt.Fprint(output, `Usage:
	mcp-server healthcheck [flags]

Flags:
`)
	flags.PrintDefaults()
}
