// Package main starts the Canterbury vault service.
package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/grpchealth"
	"connectrpc.com/grpcreflect"

	"github.com/cthierer/canterbury/gen/go/canterbury/vault/v1/vaultv1connect"
	"github.com/cthierer/canterbury/internal/adapters/auditfs"
	"github.com/cthierer/canterbury/internal/adapters/authfs"
	"github.com/cthierer/canterbury/internal/adapters/authjwt"
	"github.com/cthierer/canterbury/internal/adapters/healthgrpc"
	"github.com/cthierer/canterbury/internal/adapters/vaultfs"
	"github.com/cthierer/canterbury/internal/app/auditlog"
	appauth "github.com/cthierer/canterbury/internal/app/auth"
	apphealth "github.com/cthierer/canterbury/internal/app/health"
	vaultapp "github.com/cthierer/canterbury/internal/app/vault"
	"github.com/cthierer/canterbury/internal/cliapp"
	"github.com/cthierer/canterbury/internal/interfaces/healthcli"
	vaultconnect "github.com/cthierer/canterbury/internal/interfaces/vaultrpc"
	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

type serveConfig struct {
	Addr  string
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

type healthcheckConfig struct {
	URL     string        `envconfig:"VAULT_SERVICE_HEALTHCHECK_URL"`
	Timeout time.Duration `envconfig:"VAULT_SERVICE_HEALTHCHECK_TIMEOUT"`
}

type vaultServer struct {
	http   *http.Server
	health *grpchealth.StaticChecker
}

const (
	shutdownGracePeriod    = 10 * time.Second
	readHeaderTimeout      = 5 * time.Second
	defaultVaultServerAddr = "127.0.0.1:50051"
	defaultHealthTimeout   = 2 * time.Second
)

func main() {
	ctx := context.Background()
	exitCode := run(ctx, os.Args[1:], os.Stdout)
	os.Exit(exitCode)
}

func run(ctx context.Context, args []string, output io.Writer) int {
	app := cliapp.Application{
		Name:           "vault-service",
		DefaultCommand: "serve",
		Prepare:        loadLocalEnv,
		Commands: []cliapp.Command{
			{Name: "serve", Summary: "Start the vault service", Run: runServeCommand},
			{Name: "healthcheck", Summary: "Determine if a vault service is healthy", Run: runHealthcheckCommand},
		},
		Footer: `Run "vault-service healthcheck --help" for healthcheck flags.`,
	}

	return app.Run(ctx, args, output)
}

func runServeCommand(ctx context.Context, args []string, _ io.Writer) error {
	cfg, err := loadServeConfig(args)
	if err != nil {
		return fmt.Errorf("load vault service configuration: %w", err)
	}

	if err := serve(ctx, cfg); err != nil {
		return fmt.Errorf("vault service stopped: %w", err)
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
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	server, err := newVaultServer(ctx, cfg)
	if err != nil {
		return err
	}

	server.setServing()
	defer server.setNotServing()

	errs := make(chan error, 1)
	go func() {
		slog.InfoContext(ctx, "starting vault service", "address", cfg.Addr)
		errs <- server.http.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		server.setNotServing()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
		defer cancel()

		if err := server.http.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shut down vault service: %w", err)
		}

		return nil
	case err := <-errs:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}

		return fmt.Errorf("serve vault HTTP: %w", err)
	}
}

func (server *vaultServer) setServing() {
	server.health.SetStatus(vaultv1connect.VaultServiceName, grpchealth.StatusServing)
}

func (server *vaultServer) setNotServing() {
	server.health.SetStatus(vaultv1connect.VaultServiceName, grpchealth.StatusNotServing)
}

func newVaultServer(ctx context.Context, cfg serveConfig) (*vaultServer, error) {
	checker := grpchealth.NewStaticChecker()
	checker.SetStatus(vaultv1connect.VaultServiceName, grpchealth.StatusNotServing)

	mux := http.NewServeMux()

	vaultRepository, err := vaultfs.NewRepository(cfg.Root)
	if err != nil {
		return nil, fmt.Errorf("initialize vault repository: %w", err)
	}

	auditOptions := []auditfs.RecorderOption{}
	auditWriterID := cfg.Audit.WriterID
	if auditWriterID != "" {
		auditOptions = append(auditOptions, auditfs.WithWriterID(auditWriterID))
	}

	auditRecorder, err := auditfs.NewRecorder(cfg.Audit.Root, auditOptions...)
	if err != nil {
		return nil, fmt.Errorf("initialize audit recorder: %w", err)
	}

	auditLog, err := auditlog.NewService(auditRecorder)
	if err != nil {
		return nil, fmt.Errorf("initialize audit log: %w", err)
	}

	authMappingLoader, err := authfs.NewLoader(cfg.Auth.MappingFile)
	if err != nil {
		return nil, fmt.Errorf("initialize auth mapping loader: %w", err)
	}

	scopeMapper, err := appauth.NewScopeMapper(ctx, authMappingLoader)
	if err != nil {
		return nil, fmt.Errorf("initialize auth scope mapper: %w", err)
	}
	slog.InfoContext(
		ctx,
		"loaded auth scope mapping",
		"subjects",
		scopeMapper.SubjectCount(),
		"checksum",
		scopeMapper.MappingChecksum(),
	)

	tokenVerifier, err := authjwt.NewVerifier(ctx, cfg.Auth.JWKS.URL, []string{"EdDSA", "ES256"})
	if err != nil {
		return nil, fmt.Errorf("initialize auth JWT verifier: %w", err)
	}

	authenticator, err := appauth.NewAuthenticator(cfg.Auth.Issuer, cfg.Auth.Audience, scopeMapper, tokenVerifier)
	if err != nil {
		return nil, fmt.Errorf("initialize auth application service: %w", err)
	}

	authInterceptor, err := vaultconnect.NewAuthContextInterceptor(authenticator, auditLog)
	if err != nil {
		return nil, fmt.Errorf("initialize auth context interceptor: %w", err)
	}

	vaultApplication, err := vaultapp.NewService(vaultRepository, auditLog)
	if err != nil {
		return nil, fmt.Errorf("initialize vault application service: %w", err)
	}

	vaultService, err := vaultconnect.NewVaultServiceHandler(vaultApplication)
	if err != nil {
		return nil, fmt.Errorf("initialize vault connect service: %w", err)
	}

	auditInterceptor, err := vaultconnect.NewAuditContextInterceptor(cfg.Audit.HMACKey)
	if err != nil {
		return nil, fmt.Errorf("initialize audit context interceptor: %w", err)
	}

	vaultPath, vaultHandler := vaultv1connect.NewVaultServiceHandler(
		vaultService,
		connect.WithInterceptors(auditInterceptor, authInterceptor),
	)
	mux.Handle(vaultPath, vaultHandler)

	healthPath, healthHandler := grpchealth.NewHandler(checker)
	mux.Handle(healthPath, healthHandler)

	reflector := grpcreflect.NewStaticReflector(vaultv1connect.VaultServiceName)
	reflectV1Path, reflectV1Handler := grpcreflect.NewHandlerV1(reflector)
	mux.Handle(reflectV1Path, reflectV1Handler)
	reflectV1AlphaPath, reflectV1AlphaHandler := grpcreflect.NewHandlerV1Alpha(reflector)
	mux.Handle(reflectV1AlphaPath, reflectV1AlphaHandler)

	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)
	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           mux,
		ReadHeaderTimeout: readHeaderTimeout,
		Protocols:         protocols,
	}

	return &vaultServer{http: httpServer, health: checker}, nil
}

func healthcheck(ctx context.Context, cfg healthcheckConfig) error {
	client := &http.Client{Timeout: cfg.Timeout}
	vaultHealth := grpchealth.NewClient(client, cfg.URL)
	vaultHealthChecker, err := healthgrpc.NewChecker(vaultHealth, vaultv1connect.VaultServiceName)
	if err != nil {
		return fmt.Errorf("initialize vault health checker: %w", err)
	}

	healthApp, err := apphealth.NewService(vaultHealthChecker)
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

	cfg := serveConfig{Addr: defaultVaultServerAddr}
	if err := envconfig.Process("vault_service", &cfg); err != nil {
		return serveConfig{}, err
	}
	cfg.Addr = strings.TrimSpace(cfg.Addr)
	if cfg.Addr == "" {
		return serveConfig{}, fmt.Errorf("vault service address must not be empty")
	}

	return cfg, nil
}

func loadHealthcheckConfig(args []string, output io.Writer) (healthcheckConfig, error) {
	address := struct{ Addr string }{Addr: defaultVaultServerAddr}
	if err := envconfig.Process("vault_service", &address); err != nil {
		return healthcheckConfig{}, err
	}

	addr := strings.TrimSpace(address.Addr)
	if addr == "" {
		return healthcheckConfig{}, fmt.Errorf("vault service address must not be empty")
	}

	cfg := healthcheckConfig{URL: "http://" + addr, Timeout: defaultHealthTimeout}
	defaultURL := cfg.URL
	if err := envconfig.Process("vault_service", &cfg); err != nil {
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
			CommandName:  "vault-service healthcheck",
			URLUsage:     "Connect server base URL",
			NormalizeURL: normalizeBaseURL,
		},
	)
	if err != nil {
		return healthcheckConfig{}, err
	}

	return healthcheckConfig{URL: parsed.URL, Timeout: parsed.Timeout}, nil
}

func normalizeBaseURL(value string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
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
