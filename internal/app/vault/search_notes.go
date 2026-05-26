package vault

import (
	"context"
	"fmt"

	domainauth "github.com/cthierer/canterbury/internal/domain/auth"
	domain "github.com/cthierer/canterbury/internal/domain/vault"
)

// SearchNotes returns notes matching query within the configured principal scopes.
func (s *Service) SearchNotes(ctx context.Context, query domain.SearchNotesQuery) (domain.SearchNotesPage, error) {
	startTime := s.clock.Now()

	principal, err := principalFromContext(ctx)
	if err != nil {
		return domain.SearchNotesPage{}, fmt.Errorf("extract principal from context: %w", err)
	}

	var page domain.SearchNotesPage

	if len(principal.Scopes) == 0 {
		err := domainauth.ErrPermissionDenied
		auditErr := s.recordSearchNotesError(ctx, principal, query, err, startTime)
		if auditErr != nil {
			return domain.SearchNotesPage{}, fmt.Errorf("record audit log error: %w", auditErr)
		}

		return domain.SearchNotesPage{}, fmt.Errorf("search notes: %w", err)
	}

	query.Access = domain.AccessFilter{
		ScopesAny: principal.Scopes,
	}

	page, err = s.repository.SearchNotes(ctx, query)
	if err != nil {
		auditErr := s.recordSearchNotesError(ctx, principal, query, err, startTime)
		if auditErr != nil {
			return domain.SearchNotesPage{}, fmt.Errorf("record audit log error: %w", auditErr)
		}

		return domain.SearchNotesPage{}, fmt.Errorf("search notes: %w", err)
	}

	err = s.recordSearchNotesCompleted(ctx, principal, query, page, startTime)
	if err != nil {
		return domain.SearchNotesPage{}, fmt.Errorf("record audit log search: %w", err)
	}

	for i, result := range page.Results {
		page.Results[i].Metadata = sanitizeFrontmatter(result.Metadata)
	}

	return page, nil
}
