package vaultfs

import (
	"strings"

	"github.com/cthierer/canterbury/internal/domain/vault"
)

func matchPath(filter vault.PathFilter) matcher {
	return all(includePathPrefixes(filter.IncludePrefixes), excludePathPrefixes(filter.ExcludePrefixes))
}

func matchAnyNormalizedPathPrefix(prefixes []string) matcher {
	return func(note vault.Note) bool {
		path := strings.ToLower(note.Ref.Path.String())

		for _, prefix := range prefixes {
			if pathMatchesPrefix(path, prefix) {
				return true
			}
		}

		return false
	}
}

func includePathPrefixes(prefixes []string) matcher {
	normalizedPrefixes := normalizeStrings(prefixes)
	if len(normalizedPrefixes) == 0 {
		return always()
	}

	return matchAnyNormalizedPathPrefix(normalizedPrefixes)
}

func excludePathPrefixes(prefixes []string) matcher {
	normalizedPrefixes := normalizeStrings(prefixes)
	return not(matchAnyNormalizedPathPrefix(normalizedPrefixes))
}

func pathMatchesPrefix(pathValue, prefix string) bool {
	prefix = strings.TrimSuffix(prefix, "/")
	return pathValue == prefix || strings.HasPrefix(pathValue, prefix+"/")
}
