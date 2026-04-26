package vaultfs

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/cthierer/canterbury/internal/domain/vault"
)

func TestPaginateFromQuery(t *testing.T) {
	results := []vault.SearchNoteResult{
		searchResult(t, "A.md", time.Time{}),
		searchResult(t, "B.md", time.Time{}),
		searchResult(t, "C.md", time.Time{}),
		searchResult(t, "D.md", time.Time{}),
		searchResult(t, "E.md", time.Time{}),
	}

	tests := []struct {
		name           string
		query          vault.SearchNotesQuery
		wantPaths      []string
		wantNextCursor string
	}{
		{
			name: "returns first page and next cursor",
			query: vault.SearchNotesQuery{
				Limit: 2,
			},
			wantPaths:      []string{"A.md", "B.md"},
			wantNextCursor: "2",
		},
		{
			name: "returns middle page and next cursor",
			query: vault.SearchNotesQuery{
				Limit:  2,
				Cursor: "2",
			},
			wantPaths:      []string{"C.md", "D.md"},
			wantNextCursor: "4",
		},
		{
			name: "returns final page without next cursor",
			query: vault.SearchNotesQuery{
				Limit:  2,
				Cursor: "4",
			},
			wantPaths: []string{"E.md"},
		},
		{
			name: "returns empty page when cursor is past results",
			query: vault.SearchNotesQuery{
				Limit:  2,
				Cursor: "9",
			},
			wantPaths: []string{},
		},
		{
			name: "uses default limit",
			query: vault.SearchNotesQuery{
				Limit: 0,
			},
			wantPaths: []string{"A.md", "B.md", "C.md", "D.md", "E.md"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			paginate, err := paginateFromQuery(test.query)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			page := paginate(results)
			assertResultPaths(t, page.Results, test.wantPaths)

			if page.NextCursor != test.wantNextCursor {
				t.Fatalf("got next cursor %q, want %q", page.NextCursor, test.wantNextCursor)
			}
		})
	}
}

func TestPaginateFromQueryCapsLimit(t *testing.T) {
	results := make([]vault.SearchNoteResult, vault.DefaultSearchMaxLimit+1)
	for i := range results {
		results[i] = searchResult(t, fmt.Sprintf("Note-%03d.md", i), time.Time{})
	}

	paginate, err := paginateFromQuery(vault.SearchNotesQuery{Limit: vault.DefaultSearchMaxLimit + 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	page := paginate(results)
	if len(page.Results) != vault.DefaultSearchMaxLimit {
		t.Fatalf("got %d results, want %d", len(page.Results), vault.DefaultSearchMaxLimit)
	}

	if page.NextCursor != "200" {
		t.Fatalf("got next cursor %q, want %q", page.NextCursor, "200")
	}
}

func TestPaginateFromQueryRejectsInvalidCursor(t *testing.T) {
	tests := []struct {
		name   string
		cursor string
	}{
		{name: "non-numeric cursor", cursor: "abc"},
		{name: "negative cursor", cursor: "-1"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := paginateFromQuery(vault.SearchNotesQuery{Cursor: test.cursor})
			if !errors.Is(err, vault.ErrInvalidSearch) {
				t.Fatalf("got error %v, want %v", err, vault.ErrInvalidSearch)
			}
		})
	}
}

func TestParseCursor(t *testing.T) {
	tests := []struct {
		name    string
		cursor  string
		want    int
		wantErr bool
	}{
		{name: "empty cursor", cursor: "", want: 0},
		{name: "whitespace cursor", cursor: " \t ", want: 0},
		{name: "numeric cursor", cursor: "12", want: 12},
		{name: "trimmed numeric cursor", cursor: " 12 ", want: 12},
		{name: "invalid cursor", cursor: "nope", wantErr: true},
		{name: "negative cursor", cursor: "-2", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseCursor(test.cursor)
			if test.wantErr {
				if !errors.Is(err, vault.ErrInvalidSearch) {
					t.Fatalf("got error %v, want %v", err, vault.ErrInvalidSearch)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != test.want {
				t.Fatalf("got %d, want %d", got, test.want)
			}
		})
	}
}
