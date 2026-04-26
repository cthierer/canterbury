package vaultfs

import (
	"testing"

	"github.com/cthierer/canterbury/internal/domain/vault"
)

func TestMatchText(t *testing.T) {
	note := vault.Note{
		Content: "Canterbury keeps structured notes searchable.",
	}

	tests := []struct {
		name   string
		search vault.TextSearch
		want   bool
	}{
		{
			name: "empty filter matches",
			want: true,
		},
		{
			name: "whitespace terms are ignored",
			search: vault.TextSearch{
				Terms: []string{" ", "\t"},
			},
			want: true,
		},
		{
			name: "matches all terms case insensitively",
			search: vault.TextSearch{
				Terms: []string{"canterbury", "SEARCHABLE"},
			},
			want: true,
		},
		{
			name: "requires every term",
			search: vault.TextSearch{
				Terms: []string{"canterbury", "missing"},
			},
			want: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := matchText(test.search)(note); got != test.want {
				t.Fatalf("got %t, want %t", got, test.want)
			}
		})
	}
}
