package vault

import "context"

// Repository reads and searches notes from a backing vault.
type Repository interface {
	ReadNote(ctx context.Context, path NotePath) (Note, error)
	SearchNotes(ctx context.Context, query SearchNotesQuery) (SearchNotesPage, error)
}
