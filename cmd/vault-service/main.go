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
	"github.com/kelseyhightower/envconfig"
)

type config struct {
	Addr  string `default:"127.0.0.1:50051"`
	Root  string `required:"true"`
	Audit struct {
		Root     string  `required:"true"`
		HMACKey  HMACKey `required:"true" split_words:"true"`
		WriterID string  `split_words:"true"`
	}
	Auth struct {
		Issuer   string `required:"true"`
		Audience string `required:"true"`
		JWKS     struct {
			URL string `required:"true"`
		}
		MappingFile string `required:"true" split_words:"true"`
	}
}

const (
	shutdownGracePeriod = 10 * time.Second
	readHeaderTimeout   = 5 * time.Second
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

	var config config
	if err := envconfig.Process("vault_service", &config); err != nil {
		return err
	}

	mux := http.NewServeMux()

	vaultRepository, err := vaultfs.NewRepository(config.Root)
	if err != nil {
		return fmt.Errorf("initialize vault repository: %w", err)
	}

	auditOptions := []auditfs.RecorderOption{}
	auditWriterID := config.Audit.WriterID
	if auditWriterID != "" {
		auditOptions = append(auditOptions, auditfs.WithWriterID(auditWriterID))
	}

	auditRecorder, err := auditfs.NewRecorder(config.Audit.Root, auditOptions...)
	if err != nil {
		return fmt.Errorf("initialize audit recorder: %w", err)
	}

	auditLog, err := auditlog.NewService(auditRecorder)
	if err != nil {
		return fmt.Errorf("initialize audit log: %w", err)
	}

	authMappingLoader, err := authfs.NewLoader(config.Auth.MappingFile)
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

	tokenVerifier, err := authjwt.NewVerifier(ctx, config.Auth.JWKS.URL, []string{"EdDSA", "ES256"})
	if err != nil {
		return fmt.Errorf("initialize auth JWT verifier: %w", err)
	}

	authenticator, err := appauth.NewAuthenticator(config.Auth.Issuer, config.Auth.Audience, scopeMapper, tokenVerifier)
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

	auditInterceptor, err := vaultconnect.NewAuditContextInterceptor(config.Audit.HMACKey)
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
		Addr:              config.Addr,
		Handler:           h2c.NewHandler(mux, &http2.Server{}),
		ReadHeaderTimeout: readHeaderTimeout,
	}

	errs := make(chan error, 1)
	go func() {
		slog.InfoContext(ctx, "starting vault service", "address", config.Addr)
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

func loadLocalEnv() error {
	if err := godotenv.Load(".env"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("load dotenv configuration: %w", err)
	}

	return nil
}

type HMACKey []byte

func (hmacKey *HMACKey) Decode(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("HMAC key is required")
	}

	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return fmt.Errorf("HMAC key must be base64 encoded: %w", err)
	}

	if len(decoded) < 32 {
		return fmt.Errorf("HMAC key must decode to at least 32 bytes")
	}

	*hmacKey = decoded
	return nil
}
