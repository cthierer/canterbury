package main

import (
	"bytes"
	"errors"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cthierer/canterbury/internal/interfaces/healthcli"
)

func TestLoadServeConfigDefaultsAndOverrides(t *testing.T) {
	for _, name := range []string{
		"MCP_SERVER_ADDR",
		"MCP_SERVER_VAULT_BASE_URL",
		"MCP_SERVER_VAULT_REQUEST_TIMEOUT",
	} {
		unsetEnv(t, name)
	}

	got, err := loadServeConfig(nil)
	if err != nil {
		t.Fatalf("loadServeConfig() error = %v", err)
	}
	if got.Addr != defaultMCPServerAddr {
		t.Fatalf("Addr = %q, want default", got.Addr)
	}
	if got.Vault.BaseURL != "http://127.0.0.1:50051" {
		t.Fatalf("Vault.BaseURL = %q, want default", got.Vault.BaseURL)
	}
	if got.Vault.RequestTimeout != 10*time.Second {
		t.Fatalf("Vault.RequestTimeout = %v, want 10s", got.Vault.RequestTimeout)
	}

	t.Setenv("MCP_SERVER_ADDR", " 0.0.0.0:6000 ")
	t.Setenv("MCP_SERVER_VAULT_BASE_URL", " https://vault.example.test/ ")
	t.Setenv("MCP_SERVER_VAULT_REQUEST_TIMEOUT", "250ms")

	got, err = loadServeConfig(nil)
	if err != nil {
		t.Fatalf("loadServeConfig() override error = %v", err)
	}
	if got.Addr != "0.0.0.0:6000" || got.Vault.BaseURL != "https://vault.example.test" {
		t.Fatalf("loadServeConfig() = %+v, want normalized overrides", got)
	}
	if got.Vault.RequestTimeout != 250*time.Millisecond {
		t.Fatalf("Vault.RequestTimeout = %v, want 250ms", got.Vault.RequestTimeout)
	}
}

func TestLoadHealthcheckConfigUsesDerivedDefaults(t *testing.T) {
	for _, name := range []string{
		"MCP_SERVER_ADDR",
		"MCP_SERVER_HEALTHCHECK_URL",
		"MCP_SERVER_HEALTHCHECK_TIMEOUT",
	} {
		unsetEnv(t, name)
	}

	got, err := loadHealthcheckConfig(nil, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("loadHealthcheckConfig() error = %v", err)
	}
	if got.URL != "http://127.0.0.1:50053/health/live" {
		t.Fatalf("URL = %q, want derived default", got.URL)
	}
	if got.Timeout != defaultHealthTimeout {
		t.Fatalf("Timeout = %v, want %v", got.Timeout, defaultHealthTimeout)
	}

	t.Setenv("MCP_SERVER_ADDR", "mcp.example.test:6000")
	got, err = loadHealthcheckConfig(nil, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("loadHealthcheckConfig() address override error = %v", err)
	}
	if got.URL != "http://mcp.example.test:6000/health/live" {
		t.Fatalf("URL = %q, want derived address override", got.URL)
	}

	t.Setenv("MCP_SERVER_HEALTHCHECK_URL", " ")
	got, err = loadHealthcheckConfig(nil, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("loadHealthcheckConfig() blank URL error = %v", err)
	}
	if got.URL != "http://mcp.example.test:6000/health/live" {
		t.Fatalf("blank URL fallback = %q", got.URL)
	}
}

func TestLoadHealthcheckConfigEnvironmentAndFlags(t *testing.T) {
	t.Setenv("MCP_SERVER_HEALTHCHECK_URL", " https://environment.example.test/probe ")
	t.Setenv("MCP_SERVER_HEALTHCHECK_TIMEOUT", "250ms")

	got, err := loadHealthcheckConfig(nil, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("loadHealthcheckConfig() environment error = %v", err)
	}
	if got.URL != "https://environment.example.test/probe" || got.Timeout != 250*time.Millisecond {
		t.Fatalf("loadHealthcheckConfig() = %+v, want environment values", got)
	}

	got, err = loadHealthcheckConfig([]string{
		"--url", "http://flag.example.test/health/live",
		"--timeout", "500ms",
	}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("loadHealthcheckConfig() flags error = %v", err)
	}
	if got.URL != "http://flag.example.test/health/live" || got.Timeout != 500*time.Millisecond {
		t.Fatalf("loadHealthcheckConfig() = %+v, want flag values", got)
	}
}

func TestLoadHealthcheckConfigRejectsBadValuesAndArguments(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing scheme", args: []string{"--url", "mcp.test/health/live"}, want: "scheme must be http or https"},
		{name: "unsupported scheme", args: []string{"--url", "ftp://mcp.test/health/live"}, want: "scheme must be http or https"},
		{name: "missing host", args: []string{"--url", "http:///health/live"}, want: "host is required"},
		{name: "nonpositive timeout", args: []string{"--timeout", "0s"}, want: "timeout must be positive"},
		{name: "unexpected argument", args: []string{"unexpected"}, want: `unexpected healthcheck argument "unexpected"`},
		{name: "unknown flag", args: []string{"--unknown"}, want: "flag provided but not defined"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := loadHealthcheckConfig(test.args, &bytes.Buffer{})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("loadHealthcheckConfig() error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestLoadHealthcheckConfigShowsHelp(t *testing.T) {
	var output bytes.Buffer

	_, err := loadHealthcheckConfig([]string{"--help"}, &output)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("loadHealthcheckConfig() error = %v, want %v", err, flag.ErrHelp)
	}
	if !strings.Contains(output.String(), "mcp-server healthcheck [flags]") {
		t.Fatalf("help output = %q, want healthcheck usage", output.String())
	}
}

func TestHealthcheckReportsServingAndFailure(t *testing.T) {
	t.Run("serving", func(t *testing.T) {
		server := healthServer(t, http.StatusOK, `{"status":"SERVING"}`)

		err := healthcheck(t.Context(), healthcheckConfig{URL: server.URL, Timeout: time.Second})
		if err != nil {
			t.Fatalf("healthcheck() error = %v", err)
		}
	})

	t.Run("not serving", func(t *testing.T) {
		server := healthServer(t, http.StatusServiceUnavailable, `{"status":"NOT_SERVING"}`)

		err := healthcheck(t.Context(), healthcheckConfig{URL: server.URL, Timeout: time.Second})
		if !errors.Is(err, healthcli.ErrHealthcheckFailed) {
			t.Fatalf("healthcheck() error = %v, want %v", err, healthcli.ErrHealthcheckFailed)
		}
	})

	t.Run("HTTP failure", func(t *testing.T) {
		server := healthServer(t, http.StatusInternalServerError, `{"status":"UNKNOWN"}`)

		err := healthcheck(t.Context(), healthcheckConfig{URL: server.URL, Timeout: time.Second})
		if !errors.Is(err, healthcli.ErrHealthcheckFailed) {
			t.Fatalf("healthcheck() error = %v, want %v", err, healthcli.ErrHealthcheckFailed)
		}
	})
}

func TestHealthcheckHonorsTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		<-req.Context().Done()
	}))
	t.Cleanup(server.Close)

	err := healthcheck(t.Context(), healthcheckConfig{URL: server.URL, Timeout: 10 * time.Millisecond})
	if !errors.Is(err, healthcli.ErrHealthcheckFailed) {
		t.Fatalf("healthcheck() error = %v, want %v", err, healthcli.ErrHealthcheckFailed)
	}
}

func TestRunHealthcheckUsesExitStatus(t *testing.T) {
	t.Run("serving", func(t *testing.T) {
		server := healthServer(t, http.StatusOK, `{"status":"SERVING"}`)

		if got := run(t.Context(), []string{"healthcheck", "--url", server.URL}, &bytes.Buffer{}); got != 0 {
			t.Fatalf("run() = %d, want 0", got)
		}
	})

	t.Run("not serving", func(t *testing.T) {
		server := healthServer(t, http.StatusServiceUnavailable, `{"status":"NOT_SERVING"}`)

		if got := run(t.Context(), []string{"healthcheck", "--url", server.URL}, &bytes.Buffer{}); got != 1 {
			t.Fatalf("run() = %d, want 1", got)
		}
	})
}

func TestValidateServeConfigRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name    string
		address string
		baseURL string
		timeout time.Duration
		want    string
	}{
		{name: "empty address", address: " ", baseURL: "http://vault.test", timeout: time.Second, want: "address must not be empty"},
		{name: "missing scheme", address: "localhost:1", baseURL: "vault.test", timeout: time.Second, want: "scheme must be http or https"},
		{name: "unsupported scheme", address: "localhost:1", baseURL: "ftp://vault.test", timeout: time.Second, want: "scheme must be http or https"},
		{name: "base path", address: "localhost:1", baseURL: "https://vault.test/api", timeout: time.Second, want: "path must be empty"},
		{name: "nonpositive timeout", address: "localhost:1", baseURL: "http://vault.test", want: "timeout must be positive"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := serveConfig{Addr: test.address}
			config.Vault.BaseURL = test.baseURL
			config.Vault.RequestTimeout = test.timeout

			err := validateServeConfig(&config)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validateServeConfig() error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestLoadLocalEnv(t *testing.T) {
	dir := inTempDir(t)
	t.Setenv("MCP_SERVER_ADDR", "environment:1")
	unsetEnv(t, "MCP_SERVER_VAULT_BASE_URL")
	unsetEnv(t, "MCP_SERVER_HEALTHCHECK_URL")
	unsetEnv(t, "MCP_SERVER_HEALTHCHECK_TIMEOUT")

	err := os.WriteFile(
		filepath.Join(dir, ".env"),
		[]byte(
			"MCP_SERVER_ADDR=dotenv:1\n"+
				"MCP_SERVER_VAULT_BASE_URL=http://dotenv.test\n"+
				"MCP_SERVER_HEALTHCHECK_URL=http://health.dotenv.test/live\n"+
				"MCP_SERVER_HEALTHCHECK_TIMEOUT=250ms\n",
		),
		0o600,
	)
	if err != nil {
		t.Fatalf("write dotenv: %v", err)
	}

	if err := loadLocalEnv(); err != nil {
		t.Fatalf("loadLocalEnv() error = %v", err)
	}
	if got := os.Getenv("MCP_SERVER_ADDR"); got != "environment:1" {
		t.Fatalf("MCP_SERVER_ADDR = %q, want environment value", got)
	}
	if got := os.Getenv("MCP_SERVER_VAULT_BASE_URL"); got != "http://dotenv.test" {
		t.Fatalf("MCP_SERVER_VAULT_BASE_URL = %q, want dotenv value", got)
	}

	healthcheck, err := loadHealthcheckConfig(nil, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("loadHealthcheckConfig() error = %v", err)
	}
	if healthcheck.URL != "http://health.dotenv.test/live" || healthcheck.Timeout != 250*time.Millisecond {
		t.Fatalf("healthcheck config = %+v, want dotenv values", healthcheck)
	}
}

func healthServer(t *testing.T, statusCode int, body string) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			t.Errorf("request method = %q, want %q", req.Method, http.MethodGet)
		}
		res.WriteHeader(statusCode)
		_, _ = res.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server
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
			_ = os.Unsetenv(name)
			return
		}
		if err := os.Setenv(name, value); err != nil {
			t.Fatalf("restore environment variable %q: %v", name, err)
		}
	})
}
