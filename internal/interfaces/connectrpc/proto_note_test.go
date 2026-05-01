package connectrpc

import (
	"strings"
	"testing"
	"time"

	domainvault "github.com/cthierer/canterbury/internal/domain/vault"
)

func TestNoteToProto(t *testing.T) {
	t.Run("converts note", func(t *testing.T) {
		notePath := mustNotePath(t, "Projects/Canterbury.md")
		modifiedAt := time.Date(2026, 4, 29, 14, 30, 15, 123, time.UTC)

		note, err := noteToProto(domainvault.Note{
			Ref: domainvault.NoteRef{
				Path:  notePath,
				Title: "Canterbury",
			},
			Metadata: domainvault.NoteMetadata{
				Path:       notePath,
				Title:      "Canterbury",
				Tags:       []domainvault.Tag{"project", "ai"},
				SizeBytes:  128,
				ModifiedAt: modifiedAt,
				Frontmatter: map[string]any{
					"summary": "Public summary",
					"created": modifiedAt,
				},
			},
			Content: "# Canterbury\n",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if note.GetRef().GetPath() != notePath.String() {
			t.Fatalf("got path %q, want %q", note.GetRef().GetPath(), notePath)
		}

		if note.GetContent() != "# Canterbury\n" {
			t.Fatalf("got content %q, want note content", note.GetContent())
		}

		metadata := note.GetMetadata()
		if metadata.GetTitle() != "Canterbury" {
			t.Fatalf("got title %q, want Canterbury", metadata.GetTitle())
		}

		if metadata.GetSizeBytes() != 128 {
			t.Fatalf("got size %d, want 128", metadata.GetSizeBytes())
		}

		if !metadata.GetModifiedAt().AsTime().Equal(modifiedAt) {
			t.Fatalf("got modified_at %v, want %v", metadata.GetModifiedAt().AsTime(), modifiedAt)
		}

		tags := metadata.GetTags()
		if len(tags) != 2 || tags[0] != "project" || tags[1] != "ai" {
			t.Fatalf("got tags %#v, want project and ai", tags)
		}

		properties := metadata.GetProperties().AsMap()
		if properties["summary"] != "Public summary" {
			t.Fatalf("got summary %#v, want Public summary", properties["summary"])
		}

		if properties["created"] != modifiedAt.Format(time.RFC3339Nano) {
			t.Fatalf("got created %#v, want timestamp string", properties["created"])
		}
	})

	t.Run("returns frontmatter conversion error", func(t *testing.T) {
		notePath := mustNotePath(t, "Projects/Canterbury.md")

		_, err := noteToProto(domainvault.Note{
			Ref: domainvault.NoteRef{Path: notePath},
			Metadata: domainvault.NoteMetadata{
				Path: notePath,
				Frontmatter: map[string]any{
					"bad": map[int]string{1: "one"},
				},
			},
		})
		if err == nil {
			t.Fatal("expected error")
		}

		if !strings.Contains(err.Error(), "convert frontmatter to properties") {
			t.Fatalf("got error %q, want frontmatter conversion context", err)
		}
	})
}
