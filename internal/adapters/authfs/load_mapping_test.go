package authfs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/cthierer/canterbury/internal/domain/vault"
)

func TestLoaderLoadMapping(t *testing.T) {
	content := `[[subjects]]
issuer = "https://auth.example.test"
subject = "user_123"
scopes = ["personal-agent"]

[[subjects]]
issuer = "https://auth.example.test"
subject = "group_research"
scopes = ["personal-agent", "public-site"]
`
	loader := newTestLoader(t, content)

	got, err := loader.LoadMapping(context.Background())
	if err != nil {
		t.Fatalf("LoadMapping() error = %v", err)
	}

	if got.Checksum != testChecksum(content) {
		t.Fatalf("checksum = %q, want %q", got.Checksum, testChecksum(content))
	}

	if len(got.Subjects) != 2 {
		t.Fatalf("got %d subjects, want 2", len(got.Subjects))
	}

	if got.Subjects[0].Issuer != "https://auth.example.test" {
		t.Fatalf("first issuer = %q, want auth issuer", got.Subjects[0].Issuer)
	}

	if got.Subjects[0].Subject != "user_123" {
		t.Fatalf("first subject = %q, want user_123", got.Subjects[0].Subject)
	}

	wantFirstScopes := []vault.Scope{"personal-agent"}
	if !reflect.DeepEqual(got.Subjects[0].Scopes, wantFirstScopes) {
		t.Fatalf("first scopes = %#v, want %#v", got.Subjects[0].Scopes, wantFirstScopes)
	}

	if got.Subjects[1].Subject != "group_research" {
		t.Fatalf("second subject = %q, want group_research", got.Subjects[1].Subject)
	}

	wantSecondScopes := []vault.Scope{"personal-agent", "public-site"}
	if !reflect.DeepEqual(got.Subjects[1].Scopes, wantSecondScopes) {
		t.Fatalf("second scopes = %#v, want %#v", got.Subjects[1].Scopes, wantSecondScopes)
	}
}

func TestLoaderLoadMappingRejectsInvalidTOML(t *testing.T) {
	loader := newTestLoader(t, "[[subjects]\n")

	_, err := loader.LoadMapping(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "parse auth mapping TOML") {
		t.Fatalf("LoadMapping() error = %v, want parse context", err)
	}
}

func TestLoaderLoadMappingRejectsUnknownKeys(t *testing.T) {
	content := `[[subjects]]
issuer = "https://auth.example.test"
subject = "user_123"
scopes = ["personal-agent"]
email = "person@example.test"
`
	loader := newTestLoader(t, content)

	_, err := loader.LoadMapping(context.Background())
	if !errors.Is(err, ErrUnrecognizedKeys) {
		t.Fatalf("LoadMapping() error = %v, want %v", err, ErrUnrecognizedKeys)
	}

	if !strings.Contains(err.Error(), "email") {
		t.Fatalf("LoadMapping() error = %v, want key name", err)
	}
}

func TestLoaderLoadMappingRejectsInvalidScopes(t *testing.T) {
	content := `[[subjects]]
issuer = "https://auth.example.test"
subject = "user_123"
scopes = ["personal-agent", " "]
`
	loader := newTestLoader(t, content)

	_, err := loader.LoadMapping(context.Background())
	if !errors.Is(err, vault.ErrInvalidScope) {
		t.Fatalf("LoadMapping() error = %v, want %v", err, vault.ErrInvalidScope)
	}
}

func TestLoaderLoadMappingHonorsCanceledContext(t *testing.T) {
	loader := newTestLoader(t, "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := loader.LoadMapping(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("LoadMapping() error = %v, want %v", err, context.Canceled)
	}
}

func TestLoaderLoadMappingReturnsReadErrors(t *testing.T) {
	loader, err := NewLoader(filepath.Join(t.TempDir(), "missing.toml"))
	if err != nil {
		t.Fatalf("NewLoader() error = %v", err)
	}

	_, err = loader.LoadMapping(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("LoadMapping() error = %v, want %v", err, os.ErrNotExist)
	}
}

func TestNewLoader(t *testing.T) {
	t.Run("trims file path", func(t *testing.T) {
		loader, err := NewLoader(" ./sample-auth/scopes.toml ")
		if err != nil {
			t.Fatalf("NewLoader() error = %v", err)
		}

		if loader.filePath != "./sample-auth/scopes.toml" {
			t.Fatalf("filePath = %q, want trimmed path", loader.filePath)
		}
	})

	t.Run("rejects empty file path", func(t *testing.T) {
		_, err := NewLoader(" ")
		if !errors.Is(err, ErrInvalidFilePath) {
			t.Fatalf("NewLoader() error = %v, want %v", err, ErrInvalidFilePath)
		}
	})
}

func newTestLoader(t *testing.T, content string) *Loader {
	t.Helper()

	path := filepath.Join(t.TempDir(), "scopes.toml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write mapping file: %v", err)
	}

	loader, err := NewLoader(path)
	if err != nil {
		t.Fatalf("NewLoader() error = %v", err)
	}

	return loader
}

func testChecksum(content string) string {
	sum := sha256.Sum256([]byte(content))
	return "sha256:" + hex.EncodeToString(sum[:])
}
