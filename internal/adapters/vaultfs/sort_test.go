package vaultfs

import (
	"errors"
	"testing"
	"time"

	"github.com/cthierer/canterbury/internal/domain/vault"
)

func TestSortByQuery(t *testing.T) {
	t.Run("defaults to path ascending", func(t *testing.T) {
		results := []vault.SearchNoteResult{
			searchResult(t, "Zoo.md", time.Time{}),
			searchResult(t, "Archive.md", time.Time{}),
		}

		sortResults, err := sortByQuery(vault.SearchNotesQuery{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		sortResults(results)

		assertResultPaths(t, results, []string{"Archive.md", "Zoo.md"})
	})

	t.Run("uses modified descending", func(t *testing.T) {
		oldTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		newTime := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
		results := []vault.SearchNoteResult{
			searchResult(t, "Old.md", oldTime),
			searchResult(t, "New.md", newTime),
		}

		sortResults, err := sortByQuery(vault.SearchNotesQuery{Sort: vault.SearchSortModifiedDesc})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		sortResults(results)

		assertResultPaths(t, results, []string{"New.md", "Old.md"})
	})

	t.Run("rejects unknown sort", func(t *testing.T) {
		_, err := sortByQuery(vault.SearchNotesQuery{Sort: vault.SearchSort("unknown")})
		if !errors.Is(err, vault.ErrInvalidSearch) {
			t.Fatalf("got error %v, want %v", err, vault.ErrInvalidSearch)
		}
	})
}

func TestSortByModifiedDescTieBreaksByPath(t *testing.T) {
	modifiedAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	results := []vault.SearchNoteResult{
		searchResult(t, "B.md", modifiedAt),
		searchResult(t, "A.md", modifiedAt),
		searchResult(t, "C.md", modifiedAt.Add(time.Hour)),
	}

	sortByModifiedDesc(results)

	assertResultPaths(t, results, []string{"C.md", "A.md", "B.md"})
}

func TestSortByPathAsc(t *testing.T) {
	results := []vault.SearchNoteResult{
		searchResult(t, "Projects/Z.md", time.Time{}),
		searchResult(t, "Archive/A.md", time.Time{}),
		searchResult(t, "Projects/A.md", time.Time{}),
	}

	sortByPathAsc(results)

	assertResultPaths(t, results, []string{"Archive/A.md", "Projects/A.md", "Projects/Z.md"})
}

func searchResult(t *testing.T, path string, modifiedAt time.Time) vault.SearchNoteResult {
	t.Helper()

	notePath, err := vault.NewNotePath(path)
	if err != nil {
		t.Fatalf("create note path: %v", err)
	}

	return vault.SearchNoteResult{
		Ref: vault.NoteRef{
			Path: notePath,
		},
		Metadata: vault.NoteMetadata{
			Path:       notePath,
			ModifiedAt: modifiedAt,
		},
	}
}

func assertResultPaths(t *testing.T, results []vault.SearchNoteResult, want []string) {
	t.Helper()

	if len(results) != len(want) {
		t.Fatalf("got %d results, want %d", len(results), len(want))
	}

	for i, result := range results {
		if got := result.Ref.Path.String(); got != want[i] {
			t.Fatalf("result %d got path %q, want %q", i, got, want[i])
		}
	}
}
