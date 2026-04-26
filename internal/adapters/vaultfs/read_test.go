package vaultfs_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/cthierer/canterbury/internal/adapters/vaultfs"
	"github.com/cthierer/canterbury/internal/domain/vault"
)

const maxNoteBytes = 10 * 1024 * 1024

func TestRepositoryReadNote(t *testing.T) {
	t.Run("reads note content and file metadata", func(t *testing.T) {
		root := t.TempDir()
		notePath := mustNotePath(t, "Notes/Hello.World.md")
		content := "# Hello\n\nBody text.\n"
		writeNoteFile(t, root, notePath, content)

		repository := newTestRepository(t, root)
		note, err := repository.ReadNote(context.Background(), notePath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if note.Content != content {
			t.Fatalf("got content %q, want %q", note.Content, content)
		}

		if note.Ref.Path != notePath {
			t.Fatalf("got ref path %q, want %q", note.Ref.Path, notePath)
		}

		if note.Ref.Title != "Hello.World" {
			t.Fatalf("got ref title %q, want %q", note.Ref.Title, "Hello.World")
		}

		if note.Metadata.Path != notePath {
			t.Fatalf("got metadata path %q, want %q", note.Metadata.Path, notePath)
		}

		if note.Metadata.Title != "Hello.World" {
			t.Fatalf("got metadata title %q, want %q", note.Metadata.Title, "Hello.World")
		}

		if note.Metadata.SizeBytes != int64(len(content)) {
			t.Fatalf("got size %d, want %d", note.Metadata.SizeBytes, len(content))
		}

		if note.Metadata.ModifiedAt.IsZero() {
			t.Fatal("expected modified time")
		}
	})

	t.Run("reads frontmatter metadata and body content", func(t *testing.T) {
		root := t.TempDir()
		notePath := mustNotePath(t, "Notes/Scoped.md")
		content := "---\n" +
			"access:\n" +
			"  scopes:\n" +
			"    - personal-agent\n" +
			"    - public-site\n" +
			"tags:\n" +
			"  - project\n" +
			"  - canterbury\n" +
			"custom: value\n" +
			"---\n" +
			"# Scoped\n\nBody text.\n"
		writeNoteFile(t, root, notePath, content)
		repository := newTestRepository(t, root)

		note, err := repository.ReadNote(context.Background(), notePath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if note.Content != "# Scoped\n\nBody text.\n" {
			t.Fatalf("got content %q, want body without frontmatter", note.Content)
		}

		if !note.Metadata.HasFrontmatter {
			t.Fatal("expected frontmatter")
		}

		assertScopes(t, note.Metadata.Access.Scopes, []vault.Scope{"personal-agent", "public-site"})
		assertTags(t, note.Metadata.Tags, []vault.Tag{"project", "canterbury"})

		if got := note.Metadata.Frontmatter["custom"]; got != "value" {
			t.Fatalf("got custom frontmatter %#v, want %q", got, "value")
		}
	})

	t.Run("returns context error before reading", func(t *testing.T) {
		root := t.TempDir()
		repository := newTestRepository(t, root)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := repository.ReadNote(ctx, mustNotePath(t, "Notes/Hello.md"))
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("got error %v, want %v", err, context.Canceled)
		}
	})

	t.Run("maps missing note to domain not found", func(t *testing.T) {
		root := t.TempDir()
		repository := newTestRepository(t, root)

		_, err := repository.ReadNote(context.Background(), mustNotePath(t, "Missing.md"))
		if !errors.Is(err, vault.ErrNoteNotFound) {
			t.Fatalf("got error %v, want %v", err, vault.ErrNoteNotFound)
		}
	})

	t.Run("rejects hidden or system note path", func(t *testing.T) {
		root := t.TempDir()
		notePath := mustNotePath(t, ".obsidian/Config.md")
		writeNoteFile(t, root, notePath, "system config")
		repository := newTestRepository(t, root)

		_, err := repository.ReadNote(context.Background(), notePath)
		if !errors.Is(err, vault.ErrInvalidNotePath) {
			t.Fatalf("got error %v, want %v", err, vault.ErrInvalidNotePath)
		}
	})

	t.Run("rejects directory note path", func(t *testing.T) {
		root := t.TempDir()
		notePath := mustNotePath(t, "Directory.md")
		if err := os.Mkdir(filepath.Join(root, notePath.String()), 0o750); err != nil {
			t.Fatalf("create directory note: %v", err)
		}

		repository := newTestRepository(t, root)
		_, err := repository.ReadNote(context.Background(), notePath)
		if !errors.Is(err, vaultfs.ErrIsDirectory) {
			t.Fatalf("got error %v, want %v", err, vaultfs.ErrIsDirectory)
		}
	})

	t.Run("rejects oversized note", func(t *testing.T) {
		root := t.TempDir()
		notePath := mustNotePath(t, "Large.md")
		fullPath := filepath.Join(root, notePath.String())

		// #nosec G304 -- fullPath is built from t.TempDir and a validated NotePath.
		file, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			t.Fatalf("create large note: %v", err)
		}

		if err := file.Truncate(maxNoteBytes + 1); err != nil {
			_ = file.Close()
			t.Fatalf("truncate large note: %v", err)
		}

		if err := file.Close(); err != nil {
			t.Fatalf("close large note: %v", err)
		}

		repository := newTestRepository(t, root)
		_, err = repository.ReadNote(context.Background(), notePath)
		if !errors.Is(err, vaultfs.ErrFileTooLarge) {
			t.Fatalf("got error %v, want %v", err, vaultfs.ErrFileTooLarge)
		}
	})
}

func newTestRepository(t *testing.T, root string) *vaultfs.Repository {
	t.Helper()

	repository, err := vaultfs.NewRepository(root)
	if err != nil {
		t.Fatalf("create repository: %v", err)
	}

	return repository
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

func assertScopes(t *testing.T, got []vault.Scope, want []vault.Scope) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("got %d scopes, want %d", len(got), len(want))
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("scope %d got %q, want %q", i, got[i], want[i])
		}
	}
}

func assertTags(t *testing.T, got []vault.Tag, want []vault.Tag) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("got %d tags, want %d", len(got), len(want))
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("tag %d got %q, want %q", i, got[i], want[i])
		}
	}
}
