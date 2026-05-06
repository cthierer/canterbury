package vault

import (
	"context"
	"fmt"

	domain "github.com/cthierer/canterbury/internal/domain/vault"
)

// ReadNote returns a note when the configured principal has a matching scope.
func (s *Service) ReadNote(ctx context.Context, path domain.NotePath) (domain.Note, error) {
	startTime := s.clock.Now()

	note, err := s.repository.ReadNote(ctx, path)
	if err != nil {
		auditErr := s.recordReadNoteError(ctx, path, err, startTime)
		if auditErr != nil {
			return domain.Note{}, fmt.Errorf("record audit log error: %w", auditErr)
		}

		return domain.Note{}, fmt.Errorf("read note from repository: %w", err)
	}

	if !note.Metadata.Access.AllowsAny(s.principal.Scopes) {
		auditErr := s.recordReadNoteDenied(ctx, note, startTime)
		if auditErr != nil {
			return domain.Note{}, fmt.Errorf("record audit log denied: %w", auditErr)
		}

		return domain.Note{}, ErrPermissionDenied
	}

	err = s.recordReadNoteAllowed(ctx, note, startTime)
	if err != nil {
		return domain.Note{}, fmt.Errorf("record audit log read: %w", err)
	}

	note.Metadata = sanitizeFrontmatter(note.Metadata)

	return note, nil
}
