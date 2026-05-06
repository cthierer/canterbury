package vault_test

import (
	"testing"

	"github.com/cthierer/canterbury/internal/domain/vault"
)

func TestResourceAccessMatchedScopes(t *testing.T) {
	tests := []struct {
		name   string
		access vault.ResourceAccess
		scopes []vault.Scope
		want   []vault.Scope
	}{
		{
			name:   "returns empty when resource has no scopes",
			access: vault.ResourceAccess{Scopes: []vault.Scope{}},
			scopes: []vault.Scope{"read"},
			want:   []vault.Scope{},
		},
		{
			name:   "returns empty when caller has no scopes",
			access: vault.ResourceAccess{Scopes: []vault.Scope{"read"}},
			scopes: []vault.Scope{},
			want:   []vault.Scope{},
		},
		{
			name:   "returns empty when there is no intersection",
			access: vault.ResourceAccess{Scopes: []vault.Scope{"admin"}},
			scopes: []vault.Scope{"read", "write"},
			want:   []vault.Scope{},
		},
		{
			name:   "returns matched scope",
			access: vault.ResourceAccess{Scopes: []vault.Scope{"read", "admin"}},
			scopes: []vault.Scope{"read"},
			want:   []vault.Scope{"read"},
		},
		{
			name:   "returns all matched scopes",
			access: vault.ResourceAccess{Scopes: []vault.Scope{"read", "write", "admin"}},
			scopes: []vault.Scope{"read", "write"},
			want:   []vault.Scope{"read", "write"},
		},
		{
			name:   "deduplicates when input scopes contains duplicates",
			access: vault.ResourceAccess{Scopes: []vault.Scope{"read"}},
			scopes: []vault.Scope{"read", "read", "read"},
			want:   []vault.Scope{"read"},
		},
		{
			name:   "deduplicates across multiple duplicated scopes",
			access: vault.ResourceAccess{Scopes: []vault.Scope{"read", "write"}},
			scopes: []vault.Scope{"read", "write", "read", "write"},
			want:   []vault.Scope{"read", "write"},
		},
		{
			name:   "preserves order of first occurrence",
			access: vault.ResourceAccess{Scopes: []vault.Scope{"read", "write"}},
			scopes: []vault.Scope{"write", "read", "write"},
			want:   []vault.Scope{"write", "read"},
		},
		{
			name:   "returns empty for nil caller scopes",
			access: vault.ResourceAccess{Scopes: []vault.Scope{"read"}},
			scopes: nil,
			want:   []vault.Scope{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.access.MatchedScopes(test.scopes)

			if len(got) != len(test.want) {
				t.Fatalf("got %v (len %d), want %v (len %d)", got, len(got), test.want, len(test.want))
			}

			for i := range got {
				if got[i] != test.want[i] {
					t.Fatalf("got[%d] = %q, want %q (full: got %v, want %v)", i, got[i], test.want[i], got, test.want)
				}
			}
		})
	}
}
