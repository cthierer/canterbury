package vaultfs_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/cthierer/canterbury/internal/adapters/vaultfs"
)

func TestNewRepository(t *testing.T) {
	t.Run("accepts trimmed root", func(t *testing.T) {
		root := t.TempDir()
		notePath := mustNotePath(t, "Hello.md")
		writeNoteFile(t, root, notePath, "hello")

		repository, err := vaultfs.NewRepository(" " + root + " ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		_, err = repository.ReadNote(context.Background(), notePath)
		if err != nil {
			t.Fatalf("read note with trimmed root: %v", err)
		}
	})

	t.Run("rejects empty root", func(t *testing.T) {
		_, err := vaultfs.NewRepository(" \t ")
		if !errors.Is(err, vaultfs.ErrInvalidRoot) {
			t.Fatalf("got error %v, want %v", err, vaultfs.ErrInvalidRoot)
		}
	})

	t.Run("rejects missing root", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "missing")

		_, err := vaultfs.NewRepository(root)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestNewRepositoryResolvesSymlinkRoot(t *testing.T) {
	root := t.TempDir()
	notePath := mustNotePath(t, "Hello.md")
	writeNoteFile(t, root, notePath, "hello")
	link := filepath.Join(t.TempDir(), "vault-link")

	if err := os.Symlink(root, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	repository, err := vaultfs.NewRepository(link)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = repository.ReadNote(context.Background(), notePath)
	if err != nil {
		t.Fatalf("read note through symlink root: %v", err)
	}
}
