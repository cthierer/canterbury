package vaultfs

import (
	"strings"

	"github.com/cthierer/canterbury/internal/domain/vault"
)

func matchText(search vault.TextSearch) matcher {
	normalizedTerms := normalizeStrings(search.Terms)

	return func(note vault.Note) bool {
		if len(normalizedTerms) < 1 {
			return true
		}

		searchableText := strings.ToLower(note.Content)
		// TODO also search title and path
		for _, term := range normalizedTerms {
			if !strings.Contains(searchableText, term) {
				return false
			}
		}

		return true
	}
}
