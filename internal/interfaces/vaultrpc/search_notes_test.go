package vaultrpc

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"connectrpc.com/connect"
	vaultv1 "github.com/cthierer/canterbury/gen/go/canterbury/vault/v1"
	domainauth "github.com/cthierer/canterbury/internal/domain/auth"
	domainvault "github.com/cthierer/canterbury/internal/domain/vault"
)

func TestVaultServiceHandlerSearchNotes(t *testing.T) {
	t.Run("returns search results", func(t *testing.T) {
		notePath := mustNotePath(t)
		modifiedAt := time.Date(2026, 4, 30, 14, 30, 0, 0, time.UTC)
		var gotQuery domainvault.SearchNotesQuery

		handler := mustHandler(t, &fakeVaultApplication{
			searchNotesFunc: func(_ context.Context, query domainvault.SearchNotesQuery) (domainvault.SearchNotesPage, error) {
				gotQuery = query

				return domainvault.SearchNotesPage{
					Results: []domainvault.SearchNoteResult{
						{
							Ref: domainvault.NoteRef{
								Path:  notePath,
								Title: "Canterbury",
							},
							Metadata: domainvault.NoteMetadata{
								Path:       notePath,
								Title:      "Canterbury",
								Tags:       []domainvault.Tag{"project"},
								SizeBytes:  256,
								ModifiedAt: modifiedAt,
								Frontmatter: map[string]any{
									"summary": "Search result",
								},
							},
							Snippet: "Canterbury search result",
						},
					},
					NextCursor: "25",
				}, nil
			},
		})

		resp, err := handler.SearchNotes(context.Background(), searchNotesRequest(&vaultv1.SearchNotesRequest{
			Query: &vaultv1.SearchNotesQuery{
				Text: " Canterbury, vault ",
			},
			Filter: &vaultv1.SearchNotesFilter{
				IncludePathPrefixes: []string{"Projects"},
				ExcludePathPrefixes: []string{"Projects/Archive"},
				AllTags:             []string{"project"},
				AnyTags:             []string{"ai", "notes"},
			},
			PageSize:  25,
			PageToken: "0",
			Sort:      vaultv1.SearchSort_SEARCH_SORT_MODIFIED_DESC,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		assertSearchQuery(t, gotQuery, domainvault.SearchNotesQuery{
			Text: domainvault.TextSearch{
				Terms: []string{"Canterbury, vault"},
			},
			Path: domainvault.PathFilter{
				IncludePrefixes: []string{"Projects"},
				ExcludePrefixes: []string{"Projects/Archive"},
			},
			Tags: domainvault.TagFilter{
				All: []domainvault.Tag{"project"},
				Any: []domainvault.Tag{"ai", "notes"},
			},
			Limit:  25,
			Cursor: "0",
			Sort:   domainvault.SearchSortModifiedDesc,
		})

		if resp.Msg.GetNextPageToken() != "25" {
			t.Fatalf("got next page token %q, want 25", resp.Msg.GetNextPageToken())
		}

		results := resp.Msg.GetResults()
		if len(results) != 1 {
			t.Fatalf("got %d results, want 1", len(results))
		}

		result := results[0]
		if result.GetRef().GetPath() != notePath.String() {
			t.Fatalf("got result path %q, want %q", result.GetRef().GetPath(), notePath)
		}

		if result.GetSnippet() != "Canterbury search result" {
			t.Fatalf("got snippet %q, want search result snippet", result.GetSnippet())
		}

		metadata := result.GetMetadata()
		if metadata.GetTitle() != "Canterbury" {
			t.Fatalf("got title %q, want Canterbury", metadata.GetTitle())
		}

		if !metadata.GetModifiedAt().AsTime().Equal(modifiedAt) {
			t.Fatalf("got modified_at %v, want %v", metadata.GetModifiedAt().AsTime(), modifiedAt)
		}

		properties := metadata.GetProperties().AsMap()
		if properties["summary"] != "Search result" {
			t.Fatalf("got summary %#v, want Search result", properties["summary"])
		}
	})

	t.Run("allows empty request", func(t *testing.T) {
		var gotQuery domainvault.SearchNotesQuery
		handler := mustHandler(t, &fakeVaultApplication{
			searchNotesFunc: func(_ context.Context, query domainvault.SearchNotesQuery) (domainvault.SearchNotesPage, error) {
				gotQuery = query
				return domainvault.SearchNotesPage{}, nil
			},
		})

		_, err := handler.SearchNotes(context.Background(), searchNotesRequest(&vaultv1.SearchNotesRequest{}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		assertSearchQuery(t, gotQuery, domainvault.SearchNotesQuery{
			Sort: domainvault.SearchSortPathAsc,
		})
	})

	t.Run("ignores whitespace text", func(t *testing.T) {
		var gotQuery domainvault.SearchNotesQuery
		handler := mustHandler(t, &fakeVaultApplication{
			searchNotesFunc: func(_ context.Context, query domainvault.SearchNotesQuery) (domainvault.SearchNotesPage, error) {
				gotQuery = query
				return domainvault.SearchNotesPage{}, nil
			},
		})

		_, err := handler.SearchNotes(context.Background(), searchNotesRequest(&vaultv1.SearchNotesRequest{
			Query: &vaultv1.SearchNotesQuery{
				Text: " \t ",
			},
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		assertSearchQuery(t, gotQuery, domainvault.SearchNotesQuery{
			Sort: domainvault.SearchSortPathAsc,
		})
	})

	t.Run("maps path ascending sort", func(t *testing.T) {
		var gotQuery domainvault.SearchNotesQuery
		handler := mustHandler(t, &fakeVaultApplication{
			searchNotesFunc: func(_ context.Context, query domainvault.SearchNotesQuery) (domainvault.SearchNotesPage, error) {
				gotQuery = query
				return domainvault.SearchNotesPage{}, nil
			},
		})

		_, err := handler.SearchNotes(context.Background(), searchNotesRequest(&vaultv1.SearchNotesRequest{
			Sort: vaultv1.SearchSort_SEARCH_SORT_PATH_ASC,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		assertSearchQuery(t, gotQuery, domainvault.SearchNotesQuery{
			Sort: domainvault.SearchSortPathAsc,
		})
	})

	t.Run("rejects invalid tag", func(t *testing.T) {
		handler := mustHandler(t, &fakeVaultApplication{})

		_, err := handler.SearchNotes(context.Background(), searchNotesRequest(&vaultv1.SearchNotesRequest{
			Filter: &vaultv1.SearchNotesFilter{
				AllTags: []string{" "},
			},
		}))
		assertConnectCode(t, err, connect.CodeInvalidArgument)
	})

	t.Run("rejects invalid sort", func(t *testing.T) {
		handler := mustHandler(t, &fakeVaultApplication{})

		_, err := handler.SearchNotes(context.Background(), searchNotesRequest(&vaultv1.SearchNotesRequest{
			Sort: vaultv1.SearchSort(99),
		}))
		assertConnectCode(t, err, connect.CodeInvalidArgument)
	})

	tests := []struct {
		name      string
		searchErr error
		want      connect.Code
	}{
		{
			name:      "maps invalid search",
			searchErr: fmt.Errorf("search repository: %w", domainvault.ErrInvalidSearch),
			want:      connect.CodeInvalidArgument,
		},
		{
			name:      "maps unavailable vault",
			searchErr: fmt.Errorf("search repository: %w", domainvault.ErrVaultUnavailable),
			want:      connect.CodeUnavailable,
		},
		{
			name:      "maps missing principal",
			searchErr: fmt.Errorf("extract principal: %w", domainauth.ErrMissingPrincipal),
			want:      connect.CodeUnauthenticated,
		},
		{
			name:      "maps cancellation",
			searchErr: context.Canceled,
			want:      connect.CodeCanceled,
		},
		{
			name:      "maps deadline",
			searchErr: context.DeadlineExceeded,
			want:      connect.CodeDeadlineExceeded,
		},
		{
			name:      "maps unknown error",
			searchErr: errors.New("boom"),
			want:      connect.CodeUnknown,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := mustHandler(t, &fakeVaultApplication{
				searchNotesFunc: func(context.Context, domainvault.SearchNotesQuery) (domainvault.SearchNotesPage, error) {
					return domainvault.SearchNotesPage{}, test.searchErr
				},
			})

			_, err := handler.SearchNotes(context.Background(), searchNotesRequest(&vaultv1.SearchNotesRequest{}))
			assertConnectCode(t, err, test.want)
		})
	}

	t.Run("maps result conversion failure", func(t *testing.T) {
		notePath := mustNotePath(t)
		handler := mustHandler(t, &fakeVaultApplication{
			searchNotesFunc: func(context.Context, domainvault.SearchNotesQuery) (domainvault.SearchNotesPage, error) {
				return domainvault.SearchNotesPage{
					Results: []domainvault.SearchNoteResult{
						{
							Ref: domainvault.NoteRef{Path: notePath},
							Metadata: domainvault.NoteMetadata{
								Path: notePath,
								Frontmatter: map[string]any{
									"bad": map[int]string{1: "one"},
								},
							},
						},
					},
				}, nil
			},
		})

		_, err := handler.SearchNotes(context.Background(), searchNotesRequest(&vaultv1.SearchNotesRequest{}))
		assertConnectCode(t, err, connect.CodeUnknown)
	})
}

func searchNotesRequest(msg *vaultv1.SearchNotesRequest) *connect.Request[vaultv1.SearchNotesRequest] {
	return connect.NewRequest(msg)
}

func assertSearchQuery(t *testing.T, got, want domainvault.SearchNotesQuery) {
	t.Helper()

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got query %#v, want %#v", got, want)
	}
}
