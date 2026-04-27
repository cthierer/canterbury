package vault

import (
	"context"
	"fmt"

	domain "github.com/cthierer/canterbury/internal/domain/vault"
)

// SearchNotes returns notes matching query within the configured principal scopes.
func (s *Service) SearchNotes(ctx context.Context, query domain.SearchNotesQuery) (domain.SearchNotesPage, error) {
	query.Access = domain.AccessFilter{
		ScopesAny: s.principal.Scopes,
	}

	page, err := s.repository.SearchNotes(ctx, query)
	if err != nil {
		return domain.SearchNotesPage{}, fmt.Errorf("search notes: %w", err)
	}

	for i, result := range page.Results {
		page.Results[i].Metadata = sanitizeFrontmatter(result.Metadata)
	}

	return page, nil
}
