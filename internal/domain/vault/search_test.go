package vault_test

import (
	"testing"

	"github.com/cthierer/canterbury/internal/domain/vault"
)

func TestSearchNotesQueryNormalizedLimit(t *testing.T) {
	tests := []struct {
		name  string
		limit int
		want  int
	}{
		{name: "uses default for zero", limit: 0, want: vault.DefaultSearchLimit},
		{name: "uses default for negative", limit: -1, want: vault.DefaultSearchLimit},
		{name: "keeps requested limit", limit: 25, want: 25},
		{name: "caps large limit", limit: 500, want: vault.DefaultSearchMaxLimit},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			query := vault.SearchNotesQuery{Limit: test.limit}
			if got := query.NormalizedLimit(); got != test.want {
				t.Fatalf("got %d, want %d", got, test.want)
			}
		})
	}
}

func TestSearchNotesQueryNormalizedSort(t *testing.T) {
	query := vault.SearchNotesQuery{}
	if got := query.NormalizedSort(); got != vault.SearchSortPathAsc {
		t.Fatalf("got %q, want %q", got, vault.SearchSortPathAsc)
	}
}
