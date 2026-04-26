package vaultfs

import (
	"strings"

	"github.com/cthierer/canterbury/internal/domain/vault"
)

func matchAccess(filter vault.AccessFilter) matcher {
	return all(containsAllScopes(filter.ScopesAll), containsSomeScopes(filter.ScopesAny))
}

func containsAllScopes(scopes []vault.Scope) matcher {
	searchScopes := normalizeScopeSet(scopes)

	return func(note vault.Note) bool {
		if len(searchScopes) < 1 {
			return true
		}

		return containsAll(searchScopes, normalizeScopeSet(note.Metadata.Access.Scopes))
	}
}

func containsSomeScopes(scopes []vault.Scope) matcher {
	searchScopes := normalizeScopeSet(scopes)

	return func(note vault.Note) bool {
		if len(searchScopes) < 1 {
			return true
		}

		return containsAny(searchScopes, normalizeScopeSet(note.Metadata.Access.Scopes))
	}
}

func normalizeScopeValue(value string) string {
	return strings.TrimSpace(value)
}

func normalizeScopeSet(scopes []vault.Scope) map[string]struct{} {
	return normalizeSet(scopes, normalizeScopeValue)
}
