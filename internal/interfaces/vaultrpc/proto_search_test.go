package connectrpc

import (
	"strings"
	"testing"
	"time"

	domainvault "github.com/cthierer/canterbury/internal/domain/vault"
)

func TestSearchResultsToProto(t *testing.T) {
	t.Run("converts search results", func(t *testing.T) {
		notePath := mustNotePath(t)
		modifiedAt := time.Date(2026, 4, 30, 14, 30, 0, 0, time.UTC)

		results, err := searchResultsToProto([]domainvault.SearchNoteResult{
			{
				Ref: domainvault.NoteRef{
					Path:  notePath,
					Title: "Canterbury",
				},
				Metadata: domainvault.NoteMetadata{
					Path:       notePath,
					Title:      "Canterbury",
					Tags:       []domainvault.Tag{"project", "notes"},
					SizeBytes:  512,
					ModifiedAt: modifiedAt,
					Frontmatter: map[string]any{
						"summary": "Converted result",
					},
				},
				Snippet: "Converted search snippet",
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(results) != 1 {
			t.Fatalf("got %d results, want 1", len(results))
		}

		result := results[0]
		if result.GetRef().GetPath() != notePath.String() {
			t.Fatalf("got path %q, want %q", result.GetRef().GetPath(), notePath)
		}

		if result.GetSnippet() != "Converted search snippet" {
			t.Fatalf("got snippet %q, want converted snippet", result.GetSnippet())
		}

		metadata := result.GetMetadata()
		if metadata.GetTitle() != "Canterbury" {
			t.Fatalf("got title %q, want Canterbury", metadata.GetTitle())
		}

		if metadata.GetSizeBytes() != 512 {
			t.Fatalf("got size %d, want 512", metadata.GetSizeBytes())
		}

		tags := metadata.GetTags()
		if len(tags) != 2 || tags[0] != "project" || tags[1] != "notes" {
			t.Fatalf("got tags %#v, want project and notes", tags)
		}

		properties := metadata.GetProperties().AsMap()
		if properties["summary"] != "Converted result" {
			t.Fatalf("got summary %#v, want Converted result", properties["summary"])
		}
	})

	t.Run("returns metadata conversion error", func(t *testing.T) {
		notePath := mustNotePath(t)

		_, err := searchResultsToProto([]domainvault.SearchNoteResult{
			{
				Ref: domainvault.NoteRef{Path: notePath},
				Metadata: domainvault.NoteMetadata{
					Path: notePath,
					Frontmatter: map[string]any{
						"bad": map[int]string{1: "one"},
					},
				},
			},
		})
		if err == nil {
			t.Fatal("expected error")
		}

		if !strings.Contains(err.Error(), "build metadata proto") {
			t.Fatalf("got error %q, want metadata conversion context", err)
		}
	})
}
