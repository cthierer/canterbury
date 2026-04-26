package vaultfs

import (
	"strings"
	"testing"

	"github.com/cthierer/canterbury/internal/domain/vault"
)

func TestBuildSnippet(t *testing.T) {
	t.Run("returns empty string for empty content", func(t *testing.T) {
		got := buildSnippet(vault.Note{}, vault.TextSearch{})
		if got != "" {
			t.Fatalf("got %q, want empty string", got)
		}
	})

	t.Run("normalizes whitespace", func(t *testing.T) {
		note := vault.Note{
			Content: "First\tline\n\nsecond   line",
		}

		got := buildSnippet(note, vault.TextSearch{})
		want := "First line second line"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("returns first content when no search term is present", func(t *testing.T) {
		note := vault.Note{
			Content: strings.Repeat("a", defaultSnippetLength+1),
		}

		got := buildSnippet(note, vault.TextSearch{})

		if len(got) != defaultSnippetLength+len(snippetEllipsis) {
			t.Fatalf("got length %d, want %d", len(got), defaultSnippetLength+len(snippetEllipsis))
		}

		if !strings.HasSuffix(got, snippetEllipsis) {
			t.Fatalf("got %q, want suffix %q", got, snippetEllipsis)
		}
	})

	t.Run("centers around earliest matching term", func(t *testing.T) {
		note := vault.Note{
			Content: strings.Repeat("a", defaultSnippetLength) + " target " + strings.Repeat("b", 20),
		}

		got := buildSnippet(note, vault.TextSearch{
			Terms: []string{"target"},
		})

		if !strings.HasPrefix(got, snippetEllipsis) {
			t.Fatalf("got %q, want prefix %q", got, snippetEllipsis)
		}

		if !strings.Contains(got, "target") {
			t.Fatalf("got %q, want matching term", got)
		}
	})

	t.Run("matches terms case insensitively", func(t *testing.T) {
		note := vault.Note{
			Content: "A short Canterbury note.",
		}

		got := buildSnippet(note, vault.TextSearch{
			Terms: []string{"CANTERBURY"},
		})

		if got != "A short Canterbury note." {
			t.Fatalf("got %q, want full note", got)
		}
	})

	t.Run("does not split utf8 runes", func(t *testing.T) {
		content := strings.Repeat("é", defaultSnippetLength+1)
		note := vault.Note{
			Content: content,
		}

		got := buildSnippet(note, vault.TextSearch{})
		trimmed := strings.TrimSuffix(got, snippetEllipsis)

		if strings.ContainsRune(trimmed, utf8ReplacementRune) {
			t.Fatalf("got %q, want valid utf8 snippet", got)
		}
	})
}

func TestFirstTermIndex(t *testing.T) {
	got := firstTermIndex("alpha beta gamma", []string{"gamma", "beta"})
	if got != len("alpha ") {
		t.Fatalf("got %d, want %d", got, len("alpha "))
	}
}

const utf8ReplacementRune = '\uFFFD'
