package vault

import (
	"strings"

	domain "github.com/cthierer/canterbury/internal/domain/vault"
)

func sanitizeFrontmatter(metadata domain.NoteMetadata) domain.NoteMetadata {
	if len(metadata.Frontmatter) == 0 {
		return metadata
	}

	sanitized := make(map[string]any, len(metadata.Frontmatter))

	for key, val := range metadata.Frontmatter {
		if isReservedFrontmatterKey(key) {
			continue
		}

		sanitized[key] = val
	}

	metadata.Frontmatter = sanitized
	return metadata
}

func isReservedFrontmatterKey(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "access":
		return true
	default:
		return false
	}
}
