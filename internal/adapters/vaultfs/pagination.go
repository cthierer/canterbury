package vaultfs

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cthierer/canterbury/internal/domain/vault"
)

type paginator func(results []vault.SearchNoteResult) vault.SearchNotesPage

func paginateFromQuery(query vault.SearchNotesQuery) (paginator, error) {
	offset, err := parseCursor(query.Cursor)
	if err != nil {
		return nil, fmt.Errorf("paginate from query: %w", err)
	}

	limit := query.NormalizedLimit()

	return func(results []vault.SearchNoteResult) vault.SearchNotesPage {
		if offset >= len(results) {
			return vault.SearchNotesPage{
				Results: []vault.SearchNoteResult{},
			}
		}

		end := offset + limit
		if end > len(results) {
			end = len(results)
		}

		nextCursor := ""
		if end < len(results) {
			nextCursor = strconv.Itoa(end)
		}

		return vault.SearchNotesPage{
			Results:    results[offset:end],
			NextCursor: nextCursor,
		}
	}, nil
}

func parseCursor(cursor string) (int, error) {
	cursor = strings.TrimSpace(cursor)
	if cursor == "" {
		return 0, nil
	}

	offset, err := strconv.Atoi(cursor)
	if err != nil || offset < 0 {
		return 0, fmt.Errorf("parse search cursor %q: %w", cursor, vault.ErrInvalidSearch)
	}

	return offset, nil
}
