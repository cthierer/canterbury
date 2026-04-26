package vaultfs_test

import (
	"context"
	"errors"
	"strings"
	"testing"

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

	t.Run("returns matching notes", func(t *testing.T) {
		root := t.TempDir()
		writeNoteFile(t, root, mustNotePath(t, "Projects/Canterbury.md"), "Canterbury project notes")
		writeNoteFile(t, root, mustNotePath(t, "Journal/Today.md"), "Daily journal")
		repository := newTestRepository(t, root)

		page, err := repository.SearchNotes(context.Background(), vault.SearchNotesQuery{
			Text: vault.TextSearch{
				Terms: []string{"canterbury"},
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		assertSearchResultPaths(t, page.Results, []string{"Projects/Canterbury.md"})

		if len(page.Results) != 1 {
			t.Fatalf("got %d results, want 1", len(page.Results))
		}

		if !strings.Contains(page.Results[0].Snippet, "Canterbury") {
			t.Fatalf("got snippet %q, want matching content", page.Results[0].Snippet)
		}
	})

	t.Run("filters sorts and paginates through repository", func(t *testing.T) {
		root := t.TempDir()
		writeNoteFile(t, root, mustNotePath(t, "Projects/Bravo.md"), "Canterbury bravo notes")
		writeNoteFile(t, root, mustNotePath(t, "Projects/Alpha.md"), "Canterbury alpha notes")
		writeNoteFile(t, root, mustNotePath(t, "Archive/Old.md"), "Canterbury archived notes")
		writeNoteFile(t, root, mustNotePath(t, "Projects/Skip.txt.md"), "Unrelated content")
		repository := newTestRepository(t, root)

		page, err := repository.SearchNotes(context.Background(), vault.SearchNotesQuery{
			Text: vault.TextSearch{
				Terms: []string{"canterbury"},
			},
			Path: vault.PathFilter{
				IncludePrefixes: []string{"Projects"},
			},
			Limit: 1,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		assertSearchResultPaths(t, page.Results, []string{"Projects/Alpha.md"})
		if page.NextCursor != "1" {
			t.Fatalf("got next cursor %q, want %q", page.NextCursor, "1")
		}

		nextPage, err := repository.SearchNotes(context.Background(), vault.SearchNotesQuery{
			Text: vault.TextSearch{
				Terms: []string{"canterbury"},
			},
			Path: vault.PathFilter{
				IncludePrefixes: []string{"Projects"},
			},
			Limit:  1,
			Cursor: page.NextCursor,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		assertSearchResultPaths(t, nextPage.Results, []string{"Projects/Bravo.md"})
		if nextPage.NextCursor != "" {
			t.Fatalf("got next cursor %q, want empty cursor", nextPage.NextCursor)
		}
	})

	t.Run("skips hidden and system paths", func(t *testing.T) {
		root := t.TempDir()
		writeNoteFile(t, root, mustNotePath(t, "Projects/Visible.md"), "Canterbury visible notes")
		writeNoteFile(t, root, mustNotePath(t, ".obsidian/Config.md"), "Canterbury system notes")
		writeNoteFile(t, root, mustNotePath(t, "Projects/.Draft.md"), "Canterbury hidden file notes")
		writeNoteFile(t, root, mustNotePath(t, "Projects/.private/Plan.md"), "Canterbury hidden directory notes")
		repository := newTestRepository(t, root)

		page, err := repository.SearchNotes(context.Background(), vault.SearchNotesQuery{
			Text: vault.TextSearch{
				Terms: []string{"canterbury"},
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		assertSearchResultPaths(t, page.Results, []string{"Projects/Visible.md"})
	})

	t.Run("returns invalid search error for unsupported sort", func(t *testing.T) {
		repository := newTestRepository(t, t.TempDir())

		_, err := repository.SearchNotes(context.Background(), vault.SearchNotesQuery{
			Sort: vault.SearchSort("unknown"),
		})
		if !errors.Is(err, vault.ErrInvalidSearch) {
			t.Fatalf("got error %v, want %v", err, vault.ErrInvalidSearch)
		}
	})

	t.Run("returns invalid cursor error", func(t *testing.T) {
		repository := newTestRepository(t, t.TempDir())

		_, err := repository.SearchNotes(context.Background(), vault.SearchNotesQuery{Cursor: "nope"})
		if !errors.Is(err, vault.ErrInvalidSearch) {
			t.Fatalf("got error %v, want %v", err, vault.ErrInvalidSearch)
		}
	})
}

func assertSearchResultPaths(t *testing.T, results []vault.SearchNoteResult, want []string) {
	t.Helper()

	if len(results) != len(want) {
		t.Fatalf("got %d results, want %d", len(results), len(want))
	}

	for i, wantPath := range want {
		if got := results[i].Ref.Path.String(); got != wantPath {
			t.Fatalf("result %d got path %q, want %q", i, got, wantPath)
		}
	}
}
