package vaultfs

import (
	"context"
	"fmt"

	"github.com/cthierer/canterbury/internal/domain/vault"
)

// SearchNotes searches notes in the filesystem vault mirror.
func (r *Repository) SearchNotes(ctx context.Context, query vault.SearchNotesQuery) (vault.SearchNotesPage, error) {
	if err := ctx.Err(); err != nil {
		return vault.SearchNotesPage{}, err
	}

	_ = query

	return vault.SearchNotesPage{}, fmt.Errorf("search notes: %w", ErrNotImplemented)
}
