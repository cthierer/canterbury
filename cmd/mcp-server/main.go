// Package main starts the Canterbury MCP server.
package main

import (
	"context"
	"errors"
	"fmt"
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
	"github.com/cthierer/canterbury/gen/go/canterbury/vault/v1/vaultv1connect"
	"github.com/cthierer/canterbury/internal/interfaces/mcphttp"
	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

const (
	serverName          = "canterbury-vault"
	serverVersion       = "dev"
	readHeaderTimeout   = 5 * time.Second
	idleTimeout         = 60 * time.Second
	shutdownGracePeriod = 10 * time.Second
	serverUserAgent     = "canterbury-mcp-server/" + serverVersion
)

type config struct {
	Addr  string `default:"127.0.0.1:50053"`
	Vault struct {
		BaseURL        string        `default:"http://127.0.0.1:50051" split_words:"true"`
		RequestTimeout time.Duration `default:"10s" split_words:"true"`
	}
}

func main() {
	if err := run(); err != nil {
		slog.ErrorContext(context.Background(), "MCP server stopped", "err", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := loadLocalEnv(); err != nil {
		return err
	}

	var config config
	if err := envconfig.Process("mcp_server", &config); err != nil {
		return fmt.Errorf("load MCP server configuration: %w", err)
	}
	if err := validateConfig(&config); err != nil {
		return err
	}

	httpClient := &http.Client{Timeout: config.Vault.RequestTimeout}
	vaultClient := vaultv1connect.NewVaultServiceClient(
		httpClient,
		config.Vault.BaseURL,
		connect.WithInterceptors(mcphttp.NewForwardMetadataInterceptor(serverUserAgent)),
	)
	handler := mcphttp.NewHandler(vaultClient, serverName, serverVersion)

	requestsCtx, cancelRequests := context.WithCancel(context.Background())
	defer cancelRequests()
	server := &http.Server{
		Addr:              config.Addr,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
		BaseContext: func(net.Listener) context.Context {
			return requestsCtx
		},
	}

	errs := make(chan error, 1)
	go func() {
		slog.InfoContext(ctx, "starting MCP server", "address", config.Addr, "path", mcphttp.Path)
		errs <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
		defer cancel()

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

func validateConfig(config *config) error {
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

func loadLocalEnv() error {
	if err := godotenv.Load(".env"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("load dotenv configuration: %w", err)
	}

	return nil
}
