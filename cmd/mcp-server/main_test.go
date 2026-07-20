package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kelseyhightower/envconfig"
)

func TestConfigDefaultsAndOverrides(t *testing.T) {
	for _, name := range []string{
		"MCP_SERVER_ADDR",
		"MCP_SERVER_VAULT_BASE_URL",
		"MCP_SERVER_VAULT_REQUEST_TIMEOUT",
	} {
		unsetEnv(t, name)
	}

	got, err := processConfig()
	if err != nil {
		t.Fatalf("processConfig() error = %v", err)
	}
	if got.Addr != "127.0.0.1:50053" {
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

	got, err = processConfig()
	if err != nil {
		t.Fatalf("processConfig() override error = %v", err)
	}
	if got.Addr != "0.0.0.0:6000" || got.Vault.BaseURL != "https://vault.example.test" {
		t.Fatalf("processConfig() = %+v, want normalized overrides", got)
	}
	if got.Vault.RequestTimeout != 250*time.Millisecond {
		t.Fatalf("Vault.RequestTimeout = %v, want 250ms", got.Vault.RequestTimeout)
	}
}

func TestValidateConfigRejectsInvalidValues(t *testing.T) {
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
			config := config{Addr: test.address}
			config.Vault.BaseURL = test.baseURL
			config.Vault.RequestTimeout = test.timeout

			err := validateConfig(&config)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validateConfig() error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestLoadLocalEnv(t *testing.T) {
	dir := inTempDir(t)
	t.Setenv("MCP_SERVER_ADDR", "environment:1")
	unsetEnv(t, "MCP_SERVER_VAULT_BASE_URL")

	err := os.WriteFile(
		filepath.Join(dir, ".env"),
		[]byte("MCP_SERVER_ADDR=dotenv:1\nMCP_SERVER_VAULT_BASE_URL=http://dotenv.test\n"),
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
}

func processConfig() (config, error) {
	var config config
	if err := envconfig.Process("mcp_server", &config); err != nil {
		return config, err
	}
	if err := validateConfig(&config); err != nil {
		return config, err
	}
	return config, nil
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
