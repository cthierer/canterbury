package vaultfs

import (
	"testing"

	"github.com/cthierer/canterbury/internal/domain/vault"
)

func TestMatchAccess(t *testing.T) {
	note := vault.Note{
		Metadata: vault.NoteMetadata{
			Access: vault.ResourceAccess{
				Scopes: []vault.Scope{"personal-agent", "public-site"},
			},
		},
	}

	tests := []struct {
		name   string
		filter vault.AccessFilter
		want   bool
	}{
		{
			name: "empty filter matches",
			want: true,
		},
		{
			name: "all scopes must be present",
			filter: vault.AccessFilter{
				ScopesAll: []vault.Scope{"personal-agent", "public-site"},
			},
			want: true,
		},
		{
			name: "all scope miss rejects",
			filter: vault.AccessFilter{
				ScopesAll: []vault.Scope{"personal-agent", "missing"},
			},
			want: false,
		},
		{
			name: "any scope may be present",
			filter: vault.AccessFilter{
				ScopesAny: []vault.Scope{"missing", "public-site"},
			},
			want: true,
		},
		{
			name: "any scope miss rejects",
			filter: vault.AccessFilter{
				ScopesAny: []vault.Scope{"missing"},
			},
			want: false,
		},
		{
			name: "all and any both apply",
			filter: vault.AccessFilter{
				ScopesAll: []vault.Scope{"personal-agent"},
				ScopesAny: []vault.Scope{"missing", "public-site"},
			},
			want: true,
		},
		{
			name: "whitespace is normalized",
			filter: vault.AccessFilter{
				ScopesAll: []vault.Scope{" personal-agent "},
			},
			want: true,
		},
		{
			name: "matching is case-sensitive",
			filter: vault.AccessFilter{
				ScopesAll: []vault.Scope{"Personal-Agent"},
			},
			want: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := matchAccess(test.filter)(note); got != test.want {
				t.Fatalf("got %t, want %t", got, test.want)
			}
		})
	}
}
