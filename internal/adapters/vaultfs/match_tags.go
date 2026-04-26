package vaultfs

import (
	"strings"

	"github.com/cthierer/canterbury/internal/domain/vault"
)

func matchTags(filter vault.TagFilter) matcher {
	return all(containsAllTags(filter.All), containsSomeTags(filter.Any))
}

func containsAllTags(tags []vault.Tag) matcher {
	searchTags := normalizeTagSet(tags)

	return func(note vault.Note) bool {
		if len(searchTags) < 1 {
			return true
		}

		return containsAll(searchTags, normalizeTagSet(note.Metadata.Tags))
	}
}

func containsSomeTags(tags []vault.Tag) matcher {
	searchTags := normalizeTagSet(tags)

	return func(note vault.Note) bool {
		if len(searchTags) < 1 {
			return true
		}

		return containsAny(searchTags, normalizeTagSet(note.Metadata.Tags))
	}
}

func normalizeTagValue(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	return strings.TrimPrefix(normalized, "#")
}

func normalizeTagSet(tags []vault.Tag) map[string]struct{} {
	return normalizeSet(tags, normalizeTagValue)
}
