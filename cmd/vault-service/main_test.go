package main

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	vaultdomain "github.com/cthierer/canterbury/internal/domain/vault"
)

func TestConfigValue(t *testing.T) {
	t.Run("returns environment value", func(t *testing.T) {
		t.Setenv("CANTERBURY_TEST_VALUE", "configured")

		got := configValue("CANTERBURY_TEST_VALUE", "fallback")
		if got != "configured" {
			t.Fatalf("got %q, want configured", got)
		}
	})

	t.Run("returns fallback when unset", func(t *testing.T) {
		unsetEnv(t, "CANTERBURY_TEST_UNSET")

		got := configValue("CANTERBURY_TEST_UNSET", "fallback")
		if got != "fallback" {
			t.Fatalf("got %q, want fallback", got)
		}
	})
}

func TestRequiredConfigValue(t *testing.T) {
	t.Run("returns configured value", func(t *testing.T) {
		t.Setenv("CANTERBURY_TEST_REQUIRED", "configured")

		got, err := requiredConfigValue("CANTERBURY_TEST_REQUIRED")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got != "configured" {
			t.Fatalf("got %q, want configured", got)
		}
	})

	t.Run("rejects missing value", func(t *testing.T) {
		unsetEnv(t, "CANTERBURY_TEST_REQUIRED_MISSING")

		_, err := requiredConfigValue("CANTERBURY_TEST_REQUIRED_MISSING")
		if err == nil {
			t.Fatal("expected error")
		}

		if !strings.Contains(err.Error(), `environment variable "CANTERBURY_TEST_REQUIRED_MISSING" is required`) {
			t.Fatalf("got error %q, want required variable message", err)
		}
	})
}

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

func TestToScopes(t *testing.T) {
	t.Run("parses comma-separated scopes", func(t *testing.T) {
		got, err := toScopes(" personal-agent,public-site, ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want := []vaultdomain.Scope{"personal-agent", "public-site"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	})

	t.Run("rejects empty scopes", func(t *testing.T) {
		_, err := toScopes(" , ")
		if err == nil {
			t.Fatal("expected error")
		}

		if !strings.Contains(err.Error(), "must have at least 1 scope") {
			t.Fatalf("got error %q, want empty scopes message", err)
		}
	})
}

func TestParseHMACKey(t *testing.T) {
	validKey := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))

	t.Run("decodes base64 key", func(t *testing.T) {
		got, err := parseHMACKey(" " + validKey + " ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(got) != 32 {
			t.Fatalf("got key length %d, want 32", len(got))
		}
	})

	t.Run("rejects empty key", func(t *testing.T) {
		_, err := parseHMACKey(" ")
		if err == nil {
			t.Fatal("expected error")
		}

		if !strings.Contains(err.Error(), "HMAC key is required") {
			t.Fatalf("got error %q, want required key message", err)
		}
	})

	t.Run("rejects non-base64 key", func(t *testing.T) {
		_, err := parseHMACKey("$(openssl rand -base64 32)")
		if err == nil {
			t.Fatal("expected error")
		}

		if !strings.Contains(err.Error(), "HMAC key must be base64 encoded") {
			t.Fatalf("got error %q, want base64 message", err)
		}
	})

	t.Run("rejects short key", func(t *testing.T) {
		_, err := parseHMACKey(base64.StdEncoding.EncodeToString([]byte("short")))
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
		if !ok || key != vaultServiceAuditHMACKey {
			continue
		}

		if strings.Contains(value, "$(") {
			t.Fatalf("example HMAC key must be a literal value, got %q", value)
		}

		if _, err := parseHMACKey(value); err != nil {
			t.Fatalf("example HMAC key is not valid: %v", err)
		}

		return
	}

	t.Fatalf(".env.example missing %s", vaultServiceAuditHMACKey)
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
