// Package main starts the Canterbury vault service.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"connectrpc.com/grpchealth"
	"connectrpc.com/grpcreflect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/cthierer/canterbury/gen/go/canterbury/vault/v1/vaultv1connect"
	"github.com/cthierer/canterbury/internal/adapters/auditfs"
	"github.com/cthierer/canterbury/internal/adapters/vaultfs"
	"github.com/cthierer/canterbury/internal/app/auditlog"
	"github.com/cthierer/canterbury/internal/app/auth"
	vaultapp "github.com/cthierer/canterbury/internal/app/vault"
	vaultdomain "github.com/cthierer/canterbury/internal/domain/vault"
	vaultconnect "github.com/cthierer/canterbury/internal/interfaces/connectrpc"
	"github.com/joho/godotenv"
)

const (
	defaultAddress         = "127.0.0.1:50051"
	shutdownGracePeriod    = 10 * time.Second
	readHeaderTimeout      = 5 * time.Second
	vaultServiceAddressEnv = "VAULT_SERVICE_ADDR"
	vaultServiceRoot       = "VAULT_SERVICE_ROOT"
	vaultServiceScopes     = "VAULT_SERVICE_AUTH_SCOPES"
	vaultServiceAuditRoot  = "VAULT_SERVICE_AUDIT_ROOT"
	vaultServiceWriterID   = "VAULT_SERVICE_AUDIT_WRITER_ID"
)

func main() {
	if err := run(); err != nil {
		slog.Error("vault service stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := loadLocalEnv(); err != nil {
		return err
	}

	mux := http.NewServeMux()
	address := configValue(vaultServiceAddressEnv, defaultAddress)

	vaultRoot, err := requiredConfigValue(vaultServiceRoot)
	if err != nil {
		return fmt.Errorf("read vault configuration: %w", err)
	}

	authScopesStr, err := requiredConfigValue(vaultServiceScopes)
	if err != nil {
		return fmt.Errorf("read auth configuration: %w", err)
	}

	authScopes, err := toScopes(authScopesStr)
	if err != nil {
		return fmt.Errorf("parse auth scopes: %w", err)
	}

	vaultRepository, err := vaultfs.NewRepository(vaultRoot)
	if err != nil {
		return fmt.Errorf("initialize vault repository: %w", err)
	}

	auditRoot, err := requiredConfigValue(vaultServiceAuditRoot)
	if err != nil {
		return fmt.Errorf("read audit configuration: %w", err)
	}

	auditOptions := []auditfs.RecorderOption{}
	auditWriterID := configValue(vaultServiceWriterID, "")
	if auditWriterID != "" {
		auditOptions = append(auditOptions, auditfs.WithWriterID(auditWriterID))
	}

	auditRecorder, err := auditfs.NewRecorder(auditRoot, auditOptions...)
	if err != nil {
		return fmt.Errorf("initialize audit recorder: %w", err)
	}

	auditLog, err := auditlog.NewService(auditRecorder)
	if err != nil {
		return fmt.Errorf("initialize audit log: %w", err)
	}

	vaultApplication, err := vaultapp.NewService(vaultRepository, auth.Principal{Scopes: authScopes}, auditLog)
	if err != nil {
		return fmt.Errorf("initialize vault application service: %w", err)
	}

	vaultService, err := vaultconnect.NewVaultServiceHandler(vaultApplication)
	if err != nil {
		return fmt.Errorf("initialize vault connect service: %w", err)
	}

	vaultPath, vaultHandler := vaultv1connect.NewVaultServiceHandler(vaultService)
	mux.Handle(vaultPath, vaultHandler)

	checker := grpchealth.NewStaticChecker(vaultv1connect.VaultServiceName)
	healthPath, healthHandler := grpchealth.NewHandler(checker)
	mux.Handle(healthPath, healthHandler)

	reflector := grpcreflect.NewStaticReflector(vaultv1connect.VaultServiceName)
	reflectV1Path, reflectV1Handler := grpcreflect.NewHandlerV1(reflector)
	mux.Handle(reflectV1Path, reflectV1Handler)
	reflectV1AlphaPath, reflectV1AlphaHandler := grpcreflect.NewHandlerV1Alpha(reflector)
	mux.Handle(reflectV1AlphaPath, reflectV1AlphaHandler)

	server := &http.Server{
		Addr:              address,
		Handler:           h2c.NewHandler(mux, &http2.Server{}),
		ReadHeaderTimeout: readHeaderTimeout,
	}

	errs := make(chan error, 1)
	go func() {
		slog.Info("starting vault service", "address", address)
		errs <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}

		return nil
	case err := <-errs:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}

		return err
	}
}

func configValue(name string, fallback string) string {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}

	return value
}

func loadLocalEnv() error {
	if err := godotenv.Load(".env"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("load dotenv configuration: %w", err)
	}

	return nil
}

func requiredConfigValue(name string) (string, error) {
	value := configValue(name, "")
	if value == "" {
		return "", fmt.Errorf("environment variable %q is required", name)
	}

	return value, nil
}

func toScopes(scopesStr string) ([]vaultdomain.Scope, error) {
	scopes := strings.Split(scopesStr, ",")
	parsed := make([]vaultdomain.Scope, 0, len(scopes))

	for _, scope := range scopes {
		trimmed := strings.TrimSpace(scope)
		if trimmed == "" {
			continue
		}

		parsedScope, err := vaultdomain.NewScope(trimmed)
		if err != nil {
			return nil, fmt.Errorf("parse scope %q: %w", scope, err)
		}

		parsed = append(parsed, parsedScope)
	}

	if len(parsed) < 1 {
		return nil, errors.New("must have at least 1 scope")
	}

	return parsed, nil
}
