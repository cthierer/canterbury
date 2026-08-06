package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"flag"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"connectrpc.com/grpchealth"
	"github.com/cthierer/canterbury/gen/go/canterbury/vault/v1/vaultv1connect"
	"github.com/cthierer/canterbury/internal/interfaces/healthcli"
)

func TestLoadLocalEnv(t *testing.T) {
	t.Run("ignores missing dotenv file", func(t *testing.T) {
		inTempDir(t)

		if err := loadLocalEnv(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("loads dotenv without overriding environment", func(t *testing.T) {
		dir := inTempDir(t)
		t.Setenv("CANTERBURY_DOTENV_VALUE", "environment")
		unsetEnv(t, "CANTERBURY_DOTENV_ONLY")

		err := os.WriteFile(
			filepath.Join(dir, ".env"),
			[]byte("CANTERBURY_DOTENV_VALUE=dotenv\nCANTERBURY_DOTENV_ONLY=loaded\n"),
			0o600,
		)
		if err != nil {
			t.Fatalf("write dotenv: %v", err)
		}

		if err := loadLocalEnv(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got := os.Getenv("CANTERBURY_DOTENV_VALUE"); got != "environment" {
			t.Fatalf("got %q, want environment", got)
		}

		if got := os.Getenv("CANTERBURY_DOTENV_ONLY"); got != "loaded" {
			t.Fatalf("got %q, want loaded", got)
		}
	})
}

func TestConfigLoadsDocumentedEnvironment(t *testing.T) {
	validKey := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))

	t.Setenv("VAULT_SERVICE_ROOT", "./sample-vault")
	t.Setenv("VAULT_SERVICE_AUTH_ISSUER", "devauth.canterbury.local")
	t.Setenv("VAULT_SERVICE_AUTH_AUDIENCE", "canterbury.vault.local")
	t.Setenv("VAULT_SERVICE_AUTH_JWKS_URL", "http://127.0.0.1:50052/.well-known/jwks.json")
	t.Setenv("VAULT_SERVICE_AUTH_MAPPING_FILE", "./sample-auth/scopes.toml")
	t.Setenv("VAULT_SERVICE_AUDIT_ROOT", "./audit")
	t.Setenv("VAULT_SERVICE_AUDIT_HMAC_KEY", validKey)
	t.Setenv("VAULT_SERVICE_AUDIT_WRITER_ID", "test-writer")
	unsetEnv(t, "VAULT_SERVICE_ADDR")

	got, err := loadServeConfig(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Addr != "127.0.0.1:50051" {
		t.Fatalf("got address %q, want default address", got.Addr)
	}

	if got.Root != "./sample-vault" {
		t.Fatalf("got root %q, want ./sample-vault", got.Root)
	}

	if got.Auth.Issuer != "devauth.canterbury.local" {
		t.Fatalf("got auth issuer %q, want devauth.canterbury.local", got.Auth.Issuer)
	}

	if got.Auth.Audience != "canterbury.vault.local" {
		t.Fatalf("got auth audience %q, want canterbury.vault.local", got.Auth.Audience)
	}

	if got.Auth.JWKS.URL != "http://127.0.0.1:50052/.well-known/jwks.json" {
		t.Fatalf("got auth JWKS URL %q, want configured URL", got.Auth.JWKS.URL)
	}

	if got.Auth.MappingFile != "./sample-auth/scopes.toml" {
		t.Fatalf("got auth mapping file %q, want ./sample-auth/scopes.toml", got.Auth.MappingFile)
	}

	if got.Audit.Root != "./audit" {
		t.Fatalf("got audit root %q, want ./audit", got.Audit.Root)
	}

	if len(got.Audit.HMACKey) != 32 {
		t.Fatalf("got audit HMAC key length %d, want 32", len(got.Audit.HMACKey))
	}

	if got.Audit.WriterID != "test-writer" {
		t.Fatalf("got audit writer ID %q, want test-writer", got.Audit.WriterID)
	}
}

func TestLoadHealthcheckConfigUsesSuiteConventions(t *testing.T) {
	for _, name := range []string{
		"VAULT_SERVICE_ADDR",
		"VAULT_SERVICE_HEALTHCHECK_URL",
		"VAULT_SERVICE_HEALTHCHECK_TIMEOUT",
	} {
		unsetEnv(t, name)
	}

	got, err := loadHealthcheckConfig(nil, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("loadHealthcheckConfig() error = %v", err)
	}
	if got.URL != "http://127.0.0.1:50051" || got.Timeout != defaultHealthTimeout {
		t.Fatalf("loadHealthcheckConfig() = %+v", got)
	}

	t.Setenv("VAULT_SERVICE_ADDR", "vault.example.test:6000")
	got, err = loadHealthcheckConfig(nil, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("loadHealthcheckConfig() address error = %v", err)
	}
	if got.URL != "http://vault.example.test:6000" {
		t.Fatalf("URL = %q, want derived address", got.URL)
	}

	t.Setenv("VAULT_SERVICE_HEALTHCHECK_URL", " ")
	got, err = loadHealthcheckConfig(nil, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("loadHealthcheckConfig() blank URL error = %v", err)
	}
	if got.URL != "http://vault.example.test:6000" {
		t.Fatalf("blank URL fallback = %q", got.URL)
	}

	t.Setenv("VAULT_SERVICE_HEALTHCHECK_URL", " https://health.example.test/ ")
	t.Setenv("VAULT_SERVICE_HEALTHCHECK_TIMEOUT", "250ms")
	got, err = loadHealthcheckConfig([]string{"--url", "http://flag.example.test", "--timeout", "500ms"}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("loadHealthcheckConfig() flags error = %v", err)
	}
	if got.URL != "http://flag.example.test" || got.Timeout != 500*time.Millisecond {
		t.Fatalf("loadHealthcheckConfig() flags = %+v", got)
	}
}

func TestLoadHealthcheckConfigRejectsInvalidValuesAndShowsHelp(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{args: []string{"--url", "vault.test"}, want: "scheme must be http or https"},
		{args: []string{"--url", "http://vault.test/path"}, want: "path must be empty"},
		{args: []string{"--timeout", "0s"}, want: "timeout must be positive"},
		{args: []string{"unexpected"}, want: "unexpected healthcheck argument"},
	}
	for _, test := range tests {
		_, err := loadHealthcheckConfig(test.args, &bytes.Buffer{})
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("loadHealthcheckConfig(%q) error = %v, want containing %q", test.args, err, test.want)
		}
	}

	var output bytes.Buffer
	_, err := loadHealthcheckConfig([]string{"--help"}, &output)
	if !errors.Is(err, flag.ErrHelp) || !strings.Contains(output.String(), "vault-service healthcheck [flags]") {
		t.Fatalf("help error = %v, output = %q", err, output.String())
	}
}

func TestHealthcheckUsesNamedVaultService(t *testing.T) {
	var requestedService string
	server := newHealthServer(t, checkerFunc(func(_ context.Context, req *grpchealth.CheckRequest) (*grpchealth.CheckResponse, error) {
		requestedService = req.Service
		return &grpchealth.CheckResponse{Status: grpchealth.StatusServing}, nil
	}))

	if err := healthcheck(t.Context(), healthcheckConfig{URL: server.URL, Timeout: time.Second}); err != nil {
		t.Fatalf("healthcheck() error = %v", err)
	}
	if requestedService != vaultv1connect.VaultServiceName {
		t.Fatalf("requested service = %q, want %q", requestedService, vaultv1connect.VaultServiceName)
	}
}

func TestHealthcheckReportsFailureStatusesAndTimeout(t *testing.T) {
	for _, status := range []grpchealth.Status{grpchealth.StatusNotServing, grpchealth.StatusUnknown} {
		server := newHealthServer(t, checkerFunc(func(context.Context, *grpchealth.CheckRequest) (*grpchealth.CheckResponse, error) {
			return &grpchealth.CheckResponse{Status: status}, nil
		}))
		err := healthcheck(t.Context(), healthcheckConfig{URL: server.URL, Timeout: time.Second})
		if !errors.Is(err, healthcli.ErrHealthcheckFailed) {
			t.Fatalf("healthcheck() status %v error = %v", status, err)
		}
	}

	server := newHealthServer(t, checkerFunc(func(ctx context.Context, _ *grpchealth.CheckRequest) (*grpchealth.CheckResponse, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}))
	err := healthcheck(t.Context(), healthcheckConfig{URL: server.URL, Timeout: time.Millisecond})
	if !errors.Is(err, healthcli.ErrHealthcheckFailed) {
		t.Fatalf("healthcheck() timeout error = %v", err)
	}
}

func TestRunHealthcheckUsesExitStatus(t *testing.T) {
	server := newHealthServer(t, checkerFunc(func(context.Context, *grpchealth.CheckRequest) (*grpchealth.CheckResponse, error) {
		return &grpchealth.CheckResponse{Status: grpchealth.StatusServing}, nil
	}))
	if got := run(t.Context(), []string{"healthcheck", "--url", server.URL}, &bytes.Buffer{}); got != 0 {
		t.Fatalf("run() serving exit code = %d, want 0", got)
	}

	server = newHealthServer(t, checkerFunc(func(context.Context, *grpchealth.CheckRequest) (*grpchealth.CheckResponse, error) {
		return &grpchealth.CheckResponse{Status: grpchealth.StatusNotServing}, nil
	}))
	if got := run(t.Context(), []string{"healthcheck", "--url", server.URL}, &bytes.Buffer{}); got != 1 {
		t.Fatalf("run() not-serving exit code = %d, want 1", got)
	}
}

func TestVaultServerReadinessLifecycle(t *testing.T) {
	jwks := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		_, _ = res.Write([]byte(`{"keys":[]}`))
	}))
	t.Cleanup(jwks.Close)

	cfg := validServeConfig(t, jwks.URL)
	server, err := newVaultServer(t.Context(), cfg)
	if err != nil {
		t.Fatalf("newVaultServer() error = %v", err)
	}

	assertVaultHealth(t, server.health, grpchealth.StatusNotServing)
	httpServer := httptest.NewServer(server.http.Handler)
	t.Cleanup(httpServer.Close)
	if err := healthcheck(t.Context(), healthcheckConfig{URL: httpServer.URL, Timeout: time.Second}); !errors.Is(err, healthcli.ErrHealthcheckFailed) {
		t.Fatalf("healthcheck() before serving error = %v", err)
	}

	server.setServing()
	assertVaultHealth(t, server.health, grpchealth.StatusServing)
	if err := healthcheck(t.Context(), healthcheckConfig{URL: httpServer.URL, Timeout: time.Second}); err != nil {
		t.Fatalf("healthcheck() after serving error = %v", err)
	}
	server.setNotServing()
	assertVaultHealth(t, server.health, grpchealth.StatusNotServing)

	entries, err := os.ReadDir(cfg.Audit.Root)
	if err != nil {
		t.Fatalf("read audit root: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("health lifecycle created %d audit entries, want none", len(entries))
	}
}

func TestServeStopsWhenContextIsCanceled(t *testing.T) {
	jwks := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		_, _ = res.Write([]byte(`{"keys":[]}`))
	}))
	t.Cleanup(jwks.Close)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve listener: %v", err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("close reserved listener: %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cfg := validServeConfig(t, jwks.URL)
	cfg.Addr = address
	errs := make(chan error, 1)
	go func() {
		errs <- serve(ctx, cfg)
	}()

	healthCfg := healthcheckConfig{URL: "http://" + address, Timeout: 100 * time.Millisecond}
	for attempt := 0; attempt < 100; attempt++ {
		if err := healthcheck(t.Context(), healthCfg); err == nil {
			break
		}
		if attempt == 99 {
			t.Fatal("vault service did not become ready")
		}
		time.Sleep(5 * time.Millisecond)
	}

	cancel()

	if err := <-errs; err != nil {
		t.Fatalf("serve() canceled error = %v", err)
	}
}

func TestVaultServerFailsBeforeReadinessWhenDependenciesDoNotInitialize(t *testing.T) {
	jwks := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
		_, _ = res.Write([]byte(`{"keys":[]}`))
	}))
	t.Cleanup(jwks.Close)

	t.Run("vault", func(t *testing.T) {
		cfg := validServeConfig(t, jwks.URL)
		cfg.Root = filepath.Join(t.TempDir(), "missing")
		if _, err := newVaultServer(t.Context(), cfg); err == nil || !strings.Contains(err.Error(), "vault repository") {
			t.Fatalf("newVaultServer() error = %v", err)
		}
	})

	t.Run("audit", func(t *testing.T) {
		cfg := validServeConfig(t, jwks.URL)
		file := filepath.Join(t.TempDir(), "audit-file")
		if err := os.WriteFile(file, []byte("not a directory"), 0o600); err != nil {
			t.Fatalf("write audit file: %v", err)
		}
		cfg.Audit.Root = file
		if _, err := newVaultServer(t.Context(), cfg); err == nil || !strings.Contains(err.Error(), "audit recorder") {
			t.Fatalf("newVaultServer() error = %v", err)
		}
	})

	t.Run("mapping", func(t *testing.T) {
		cfg := validServeConfig(t, jwks.URL)
		cfg.Auth.MappingFile = filepath.Join(t.TempDir(), "missing.toml")
		if _, err := newVaultServer(t.Context(), cfg); err == nil || !strings.Contains(err.Error(), "scope mapper") {
			t.Fatalf("newVaultServer() error = %v", err)
		}
	})

	t.Run("JWKS", func(t *testing.T) {
		failedJWKS := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
			http.Error(res, "unavailable", http.StatusServiceUnavailable)
		}))
		t.Cleanup(failedJWKS.Close)

		cfg := validServeConfig(t, failedJWKS.URL)
		if _, err := newVaultServer(t.Context(), cfg); err == nil || !strings.Contains(err.Error(), "JWT verifier") {
			t.Fatalf("newVaultServer() error = %v", err)
		}
	})
}

func TestHMACKeyDecode(t *testing.T) {
	validKey := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))

	t.Run("decodes base64 key", func(t *testing.T) {
		var got HMACKey
		err := got.Decode(" " + validKey + " ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(got) != 32 {
			t.Fatalf("got key length %d, want 32", len(got))
		}
	})

	t.Run("rejects empty key", func(t *testing.T) {
		var got HMACKey
		err := got.Decode(" ")
		if err == nil {
			t.Fatal("expected error")
		}

		if !strings.Contains(err.Error(), "HMAC key is required") {
			t.Fatalf("got error %q, want required key message", err)
		}
	})

	t.Run("rejects non-base64 key", func(t *testing.T) {
		var got HMACKey
		err := got.Decode("$(openssl rand -base64 32)")
		if err == nil {
			t.Fatal("expected error")
		}

		if !strings.Contains(err.Error(), "HMAC key must be base64 encoded") {
			t.Fatalf("got error %q, want base64 message", err)
		}
	})

	t.Run("rejects short key", func(t *testing.T) {
		var got HMACKey
		err := got.Decode(base64.StdEncoding.EncodeToString([]byte("short")))
		if err == nil {
			t.Fatal("expected error")
		}

		if !strings.Contains(err.Error(), "HMAC key must decode to at least 32 bytes") {
			t.Fatalf("got error %q, want minimum length message", err)
		}
	})
}

func TestEnvExampleHMACKey(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", ".env.example"))
	if err != nil {
		t.Fatalf("read .env.example: %v", err)
	}

	for _, line := range strings.Split(string(content), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok || key != "VAULT_SERVICE_AUDIT_HMAC_KEY" {
			continue
		}

		if strings.Contains(value, "$(") {
			t.Fatalf("example HMAC key must be a literal value, got %q", value)
		}

		var hmacKey HMACKey
		if err := hmacKey.Decode(value); err != nil {
			t.Fatalf("example HMAC key is not valid: %v", err)
		}

		return
	}

	t.Fatal(".env.example missing VAULT_SERVICE_AUDIT_HMAC_KEY")
}

func inTempDir(t *testing.T) string {
	t.Helper()

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}

	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("change working directory: %v", err)
	}

	t.Cleanup(func() {
		if err := os.Chdir(oldDir); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	})

	return dir
}

func unsetEnv(t *testing.T, name string) {
	t.Helper()

	value, existed := os.LookupEnv(name)
	if err := os.Unsetenv(name); err != nil {
		t.Fatalf("unset environment variable %q: %v", name, err)
	}

	t.Cleanup(func() {
		if !existed {
			if err := os.Unsetenv(name); err != nil {
				t.Fatalf("restore environment variable %q: %v", name, err)
			}

			return
		}

		if err := os.Setenv(name, value); err != nil {
			t.Fatalf("restore environment variable %q: %v", name, err)
		}
	})
}

type checkerFunc func(context.Context, *grpchealth.CheckRequest) (*grpchealth.CheckResponse, error)

func (check checkerFunc) Check(ctx context.Context, req *grpchealth.CheckRequest) (*grpchealth.CheckResponse, error) {
	return check(ctx, req)
}

func newHealthServer(t *testing.T, checker grpchealth.Checker) *httptest.Server {
	t.Helper()

	path, handler := grpchealth.NewHandler(checker)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func validServeConfig(t *testing.T, jwksURL string) serveConfig {
	t.Helper()

	vaultRoot := t.TempDir()
	auditRoot := t.TempDir()
	mappingPath := filepath.Join(t.TempDir(), "scopes.toml")
	mapping := `[[subjects]]
issuer = "https://auth.example.test"
subject = "user_123"
scopes = ["personal-agent"]
`
	if err := os.WriteFile(mappingPath, []byte(mapping), 0o600); err != nil {
		t.Fatalf("write mapping: %v", err)
	}

	cfg := serveConfig{Addr: defaultVaultServerAddr, Root: vaultRoot}
	cfg.Audit.Root = auditRoot
	cfg.Audit.HMACKey = []byte("0123456789abcdef0123456789abcdef")
	cfg.Audit.WriterID = "test-writer"
	cfg.Auth.Issuer = "https://auth.example.test"
	cfg.Auth.Audience = "https://vault.example.test"
	cfg.Auth.JWKS.URL = jwksURL
	cfg.Auth.MappingFile = mappingPath
	return cfg
}

func assertVaultHealth(t *testing.T, checker *grpchealth.StaticChecker, want grpchealth.Status) {
	t.Helper()

	response, err := checker.Check(t.Context(), &grpchealth.CheckRequest{Service: vaultv1connect.VaultServiceName})
	if err != nil {
		t.Fatalf("health Check() error = %v", err)
	}
	if response.Status != want {
		t.Fatalf("health status = %v, want %v", response.Status, want)
	}
}
