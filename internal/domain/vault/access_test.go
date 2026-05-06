package vault_test

import (
	"testing"

	"github.com/cthierer/canterbury/internal/domain/vault"
)

func TestResourceAccessMatchedScopes(t *testing.T) {
	t.Run("returns empty when resource has no scopes", func(t *testing.T) {
		access := vault.ResourceAccess{Scopes: []vault.Scope{}}
		got := access.MatchedScopes([]vault.Scope{"read"})
		if len(got) != 0 {
			t.Fatalf("got %v, want empty", got)
		}
	})

	t.Run("returns empty when caller has no scopes", func(t *testing.T) {
		access := vault.ResourceAccess{Scopes: []vault.Scope{"read"}}
		got := access.MatchedScopes([]vault.Scope{})
		if len(got) != 0 {
			t.Fatalf("got %v, want empty", got)
		}
	})

	t.Run("returns empty when there is no intersection", func(t *testing.T) {
		access := vault.ResourceAccess{Scopes: []vault.Scope{"admin"}}
		got := access.MatchedScopes([]vault.Scope{"read", "write"})
		if len(got) != 0 {
			t.Fatalf("got %v, want empty", got)
		}
	})

	t.Run("returns matched scope", func(t *testing.T) {
		access := vault.ResourceAccess{Scopes: []vault.Scope{"read", "admin"}}
		got := access.MatchedScopes([]vault.Scope{"read"})
		if len(got) != 1 || got[0] != "read" {
			t.Fatalf("got %v, want [read]", got)
		}
	})

	t.Run("returns all matched scopes", func(t *testing.T) {
		access := vault.ResourceAccess{Scopes: []vault.Scope{"read", "write", "admin"}}
		got := access.MatchedScopes([]vault.Scope{"read", "write"})
		if len(got) != 2 || got[0] != "read" || got[1] != "write" {
			t.Fatalf("got %v, want [read write]", got)
		}
	})

	t.Run("deduplicates when input scopes contains duplicates", func(t *testing.T) {
		access := vault.ResourceAccess{Scopes: []vault.Scope{"read"}}
		got := access.MatchedScopes([]vault.Scope{"read", "read", "read"})
		if len(got) != 1 || got[0] != "read" {
			t.Fatalf("got %v, want [read]", got)
		}
	})

	t.Run("deduplicates across multiple duplicated scopes", func(t *testing.T) {
		access := vault.ResourceAccess{Scopes: []vault.Scope{"read", "write"}}
		got := access.MatchedScopes([]vault.Scope{"read", "write", "read", "write"})
		if len(got) != 2 || got[0] != "read" || got[1] != "write" {
			t.Fatalf("got %v, want [read write]", got)
		}
	})

	t.Run("preserves order of first occurrence", func(t *testing.T) {
		access := vault.ResourceAccess{Scopes: []vault.Scope{"read", "write"}}
		got := access.MatchedScopes([]vault.Scope{"write", "read", "write"})
		if len(got) != 2 || got[0] != "write" || got[1] != "read" {
			t.Fatalf("got %v, want [write read]", got)
		}
	})

	t.Run("returns empty for nil caller scopes", func(t *testing.T) {
		access := vault.ResourceAccess{Scopes: []vault.Scope{"read"}}
		got := access.MatchedScopes(nil)
		if len(got) != 0 {
			t.Fatalf("got %v, want empty", got)
		}
	})
}
