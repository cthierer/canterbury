package vault

import (
	"context"
	"fmt"

	domain "github.com/cthierer/canterbury/internal/domain/vault"
)

// ReadNote returns a note when the configured principal has a matching scope.
func (s *Service) ReadNote(ctx context.Context, path domain.NotePath) (domain.Note, error) {
	note, err := s.repository.ReadNote(ctx, path)
	if err != nil {
		return domain.Note{}, fmt.Errorf("read note from repository: %w", err)
	}

	if !note.Metadata.Access.AllowsAny(s.principal.Scopes) {
		return domain.Note{}, ErrPermissionDenied
	}

	note.Metadata = sanitizeFrontmatter(note.Metadata)

	return note, nil
}
