package vaultfs_test

import (
	"context"
	"errors"
	"testing"

	"github.com/cthierer/canterbury/internal/adapters/vaultfs"
	"github.com/cthierer/canterbury/internal/domain/vault"
)

func TestRepositorySearchNotes(t *testing.T) {
	t.Run("returns context error before search", func(t *testing.T) {
		repository := newTestRepository(t, t.TempDir())
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := repository.SearchNotes(ctx, vault.SearchNotesQuery{})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("got error %v, want %v", err, context.Canceled)
		}
	})

	t.Run("returns not implemented", func(t *testing.T) {
		repository := newTestRepository(t, t.TempDir())

		_, err := repository.SearchNotes(context.Background(), vault.SearchNotesQuery{})
		if !errors.Is(err, vaultfs.ErrNotImplemented) {
			t.Fatalf("got error %v, want %v", err, vaultfs.ErrNotImplemented)
		}
	})
}
