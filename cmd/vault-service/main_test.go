package main

import (
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
