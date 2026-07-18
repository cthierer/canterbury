package main

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kelseyhightower/envconfig"
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

	var got config
	if err := envconfig.Process("vault_service", &got); err != nil {
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
