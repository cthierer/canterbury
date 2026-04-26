package vaultfs

import (
	"github.com/cthierer/canterbury/internal/domain/vault"
)

type matcher func(note vault.Note) bool

func matchQuery(query vault.SearchNotesQuery) matcher {
	return all(
		matchText(query.Text),
		matchPath(query.Path),
		matchTags(query.Tags),
		matchAccess(query.Access),
	)
}

func all(matchers ...matcher) matcher {
	return func(note vault.Note) bool {
		for _, matches := range matchers {
			if !matches(note) {
				return false
			}
		}

		return true
	}
}

func always() matcher {
	return func(_ vault.Note) bool {
		return true
	}
}

func not(match matcher) matcher {
	return func(note vault.Note) bool {
		return !match(note)
	}
}
