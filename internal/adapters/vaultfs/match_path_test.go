package vaultfs

import (
	"testing"

	"github.com/cthierer/canterbury/internal/domain/vault"
)

func TestMatchPath(t *testing.T) {
	note := noteWithPath(t, "Projects/Canterbury/Plan.md")

	tests := []struct {
		name   string
		filter vault.PathFilter
		want   bool
	}{
		{
			name: "empty filter matches",
			want: true,
		},
		{
			name: "include prefix matches",
			filter: vault.PathFilter{
				IncludePrefixes: []string{"projects/canterbury"},
			},
			want: true,
		},
		{
			name: "include prefixes are any-of",
			filter: vault.PathFilter{
				IncludePrefixes: []string{"journal", "projects/canterbury"},
			},
			want: true,
		},
		{
			name: "include prefix miss rejects",
			filter: vault.PathFilter{
				IncludePrefixes: []string{"journal"},
			},
			want: false,
		},
		{
			name: "exclude prefix rejects",
			filter: vault.PathFilter{
				ExcludePrefixes: []string{"projects"},
			},
			want: false,
		},
		{
			name: "exclude beats include",
			filter: vault.PathFilter{
				IncludePrefixes: []string{"projects"},
				ExcludePrefixes: []string{"projects/canterbury"},
			},
			want: false,
		},
		{
			name: "ignores whitespace-only prefixes",
			filter: vault.PathFilter{
				IncludePrefixes: []string{" "},
			},
			want: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := matchPath(test.filter)(note); got != test.want {
				t.Fatalf("got %t, want %t", got, test.want)
			}
		})
	}
}

func TestPathMatchesPrefix(t *testing.T) {
	tests := []struct {
		name      string
		pathValue string
		prefix    string
		want      bool
	}{
		{
			name:      "exact file path matches",
			pathValue: "projects/canterbury.md",
			prefix:    "projects/canterbury.md",
			want:      true,
		},
		{
			name:      "directory prefix matches child",
			pathValue: "projects/canterbury/plan.md",
			prefix:    "projects",
			want:      true,
		},
		{
			name:      "trailing slash prefix matches child",
			pathValue: "projects/canterbury/plan.md",
			prefix:    "projects/",
			want:      true,
		},
		{
			name:      "sibling prefix does not match",
			pathValue: "projects-old/plan.md",
			prefix:    "projects",
			want:      false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := pathMatchesPrefix(test.pathValue, test.prefix); got != test.want {
				t.Fatalf("got %t, want %t", got, test.want)
			}
		})
	}
}

func noteWithPath(t *testing.T, value string) vault.Note {
	t.Helper()

	notePath, err := vault.NewNotePath(value)
	if err != nil {
		t.Fatalf("create note path: %v", err)
	}

	return vault.Note{
		Ref: vault.NoteRef{
			Path: notePath,
		},
		Metadata: vault.NoteMetadata{
			Path: notePath,
		},
	}
}
