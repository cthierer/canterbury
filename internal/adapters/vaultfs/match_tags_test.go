package vaultfs

import (
	"testing"

	"github.com/cthierer/canterbury/internal/domain/vault"
)

func TestMatchTags(t *testing.T) {
	note := vault.Note{
		Metadata: vault.NoteMetadata{
			Tags: []vault.Tag{"Project", "canterbury", "AI"},
		},
	}

	tests := []struct {
		name   string
		filter vault.TagFilter
		want   bool
	}{
		{
			name: "empty filter matches",
			want: true,
		},
		{
			name: "all tags must be present",
			filter: vault.TagFilter{
				All: []vault.Tag{"project", "ai"},
			},
			want: true,
		},
		{
			name: "all tag miss rejects",
			filter: vault.TagFilter{
				All: []vault.Tag{"project", "missing"},
			},
			want: false,
		},
		{
			name: "any tag may be present",
			filter: vault.TagFilter{
				Any: []vault.Tag{"missing", "canterbury"},
			},
			want: true,
		},
		{
			name: "any tag miss rejects",
			filter: vault.TagFilter{
				Any: []vault.Tag{"missing"},
			},
			want: false,
		},
		{
			name: "all and any both apply",
			filter: vault.TagFilter{
				All: []vault.Tag{"project"},
				Any: []vault.Tag{"missing", "ai"},
			},
			want: true,
		},
		{
			name: "hash prefix is ignored",
			filter: vault.TagFilter{
				All: []vault.Tag{"#project"},
			},
			want: true,
		},
		{
			name: "case and whitespace are normalized",
			filter: vault.TagFilter{
				All: []vault.Tag{" #PROJECT "},
			},
			want: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := matchTags(test.filter)(note); got != test.want {
				t.Fatalf("got %t, want %t", got, test.want)
			}
		})
	}
}

func TestNormalizeTagValue(t *testing.T) {
	if got := normalizeTagValue(" #Project "); got != "project" {
		t.Fatalf("got %q, want %q", got, "project")
	}
}
