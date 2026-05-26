// Package main starts the Canterbury vault service.
package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/grpchealth"
	"connectrpc.com/grpcreflect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/cthierer/canterbury/gen/go/canterbury/vault/v1/vaultv1connect"
	"github.com/cthierer/canterbury/internal/adapters/auditfs"
	"github.com/cthierer/canterbury/internal/adapters/authfs"
	"github.com/cthierer/canterbury/internal/adapters/authjwt"
	"github.com/cthierer/canterbury/internal/adapters/vaultfs"
	"github.com/cthierer/canterbury/internal/app/auditlog"
	appauth "github.com/cthierer/canterbury/internal/app/auth"
	vaultapp "github.com/cthierer/canterbury/internal/app/vault"
	vaultconnect "github.com/cthierer/canterbury/internal/interfaces/vaultrpc"
	"github.com/joho/godotenv"
)

const (
	defaultAddress           = "127.0.0.1:50051"
	shutdownGracePeriod      = 10 * time.Second
	readHeaderTimeout        = 5 * time.Second
	vaultServiceAddressEnv   = "VAULT_SERVICE_ADDR"
	vaultServiceRoot         = "VAULT_SERVICE_ROOT"
	vaultServiceAuthIssuer   = "VAULT_SERVICE_AUTH_ISSUER"
	vaultServiceAuthAudience = "VAULT_SERVICE_AUTH_AUDIENCE"
	vaultServiceAuthJWKSURL  = "VAULT_SERVICE_AUTH_JWKS_URL"
	vaultServiceAuthMapping  = "VAULT_SERVICE_AUTH_MAPPING_FILE"
	vaultServiceAuditRoot    = "VAULT_SERVICE_AUDIT_ROOT"
	vaultServiceAuditHMACKey = "VAULT_SERVICE_AUDIT_HMAC_KEY"
	vaultServiceWriterID     = "VAULT_SERVICE_AUDIT_WRITER_ID"
)

func main() {
	if err := run(); err != nil {
		slog.ErrorContext(context.Background(), "vault service stopped", "err", err)
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

	authIssuer, err := requiredConfigValue(vaultServiceAuthIssuer)
	if err != nil {
		return fmt.Errorf("read auth issuer configuration: %w", err)
	}

	authAudience, err := requiredConfigValue(vaultServiceAuthAudience)
	if err != nil {
		return fmt.Errorf("read auth audience configuration: %w", err)
	}

	authJWKSURL, err := requiredConfigValue(vaultServiceAuthJWKSURL)
	if err != nil {
		return fmt.Errorf("read auth JWKS configuration: %w", err)
	}

	authMappingFile, err := requiredConfigValue(vaultServiceAuthMapping)
	if err != nil {
		return fmt.Errorf("read auth mapping configuration: %w", err)
	}

	authMappingLoader, err := authfs.NewLoader(authMappingFile)
	if err != nil {
		return fmt.Errorf("initialize auth mapping loader: %w", err)
	}

	scopeMapper, err := appauth.NewScopeMapper(ctx, authMappingLoader)
	if err != nil {
		return fmt.Errorf("initialize auth scope mapper: %w", err)
	}
	slog.InfoContext(
		ctx,
		"loaded auth scope mapping",
		"subjects",
		scopeMapper.SubjectCount(),
		"checksum",
		scopeMapper.MappingChecksum(),
	)

	tokenVerifier, err := authjwt.NewVerifier(ctx, authJWKSURL, []string{"EdDSA", "ES256"})
	if err != nil {
		return fmt.Errorf("initialize auth JWT verifier: %w", err)
	}

	authenticator, err := appauth.NewAuthenticator(authIssuer, authAudience, scopeMapper, tokenVerifier)
	if err != nil {
		return fmt.Errorf("initialize auth application service: %w", err)
	}

	authInterceptor, err := vaultconnect.NewAuthContextInterceptor(authenticator, auditLog)
	if err != nil {
		return fmt.Errorf("initialize auth context interceptor: %w", err)
	}

	vaultApplication, err := vaultapp.NewService(vaultRepository, auditLog)
	if err != nil {
		return fmt.Errorf("initialize vault application service: %w", err)
	}

	vaultService, err := vaultconnect.NewVaultServiceHandler(vaultApplication)
	if err != nil {
		return fmt.Errorf("initialize vault connect service: %w", err)
	}

	auditHMACKeyBase64, err := requiredConfigValue(vaultServiceAuditHMACKey)
	if err != nil {
		return fmt.Errorf("read audit hmac configuration: %w", err)
	}

	auditHMACKey, err := parseHMACKey(auditHMACKeyBase64)
	if err != nil {
		return fmt.Errorf("parse audit hmac key: %w", err)
	}

	auditInterceptor, err := vaultconnect.NewAuditContextInterceptor(auditHMACKey)
	if err != nil {
		return fmt.Errorf("initialize audit context interceptor: %w", err)
	}

	vaultPath, vaultHandler := vaultv1connect.NewVaultServiceHandler(
		vaultService,
		connect.WithInterceptors(auditInterceptor, authInterceptor),
	)
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
		slog.InfoContext(ctx, "starting vault service", "address", address)
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

func parseHMACKey(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("HMAC key is required")
	}

	key, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("HMAC key must be base64 encoded: %w", err)
	}

	if len(key) < 32 {
		return nil, fmt.Errorf("HMAC key must decode to at least 32 bytes")
	}

	return key, nil
}
