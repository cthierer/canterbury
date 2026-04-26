package vaultfs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/cthierer/canterbury/internal/domain/vault"
)

func TestJoinVaultPath(t *testing.T) {
	t.Run("returns resolved file path inside root", func(t *testing.T) {
		root := t.TempDir()
		notePath := mustNotePath(t, "Notes/Hello.md")
		expectedPath := writeNoteFile(t, root, notePath, "hello")

		got, err := joinVaultPath(root, notePath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got != expectedPath {
			t.Fatalf("got path %q, want %q", got, expectedPath)
		}
	})

	t.Run("maps missing file to domain not found", func(t *testing.T) {
		root := t.TempDir()
		notePath := mustNotePath(t, "Missing.md")

		_, err := joinVaultPath(root, notePath)
		if !errors.Is(err, vault.ErrNoteNotFound) {
			t.Fatalf("got error %v, want %v", err, vault.ErrNoteNotFound)
		}
	})

	t.Run("rejects symlink escape", func(t *testing.T) {
		root := t.TempDir()
		outside := filepath.Join(t.TempDir(), "Outside.md")
		if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
			t.Fatalf("write outside file: %v", err)
		}

		linkPath := filepath.Join(root, "Linked.md")
		if err := os.Symlink(outside, linkPath); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}

		_, err := joinVaultPath(root, mustNotePath(t, "Linked.md"))
		if !errors.Is(err, vault.ErrInvalidNotePath) {
			t.Fatalf("got error %v, want %v", err, vault.ErrInvalidNotePath)
		}
	})
}

func TestIsWithinRoot(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "tmp", "vault")

	tests := []struct {
		name      string
		candidate string
		want      bool
	}{
		{
			name:      "same path",
			candidate: root,
			want:      true,
		},
		{
			name:      "child path",
			candidate: filepath.Join(root, "Notes", "Hello.md"),
			want:      true,
		},
		{
			name:      "sibling prefix path",
			candidate: filepath.Join(string(filepath.Separator), "tmp", "vault-other", "Hello.md"),
			want:      false,
		},
		{
			name:      "parent path",
			candidate: filepath.Dir(root),
			want:      false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isWithinRoot(root, test.candidate); got != test.want {
				t.Fatalf("got %t, want %t", got, test.want)
			}
		})
	}
}

func writeNoteFile(t *testing.T, root string, notePath vault.NotePath, content string) string {
	t.Helper()

	fullPath := filepath.Join(root, filepath.FromSlash(notePath.String()))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o750); err != nil {
		t.Fatalf("create note directory: %v", err)
	}

	if err := os.WriteFile(fullPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write note file: %v", err)
	}

	resolvedPath, err := filepath.EvalSymlinks(fullPath)
	if err != nil {
		t.Fatalf("resolve note path: %v", err)
	}

	return resolvedPath
}

func mustNotePath(t *testing.T, value string) vault.NotePath {
	t.Helper()

	notePath, err := vault.NewNotePath(value)
	if err != nil {
		t.Fatalf("create note path: %v", err)
	}

	return notePath
}
