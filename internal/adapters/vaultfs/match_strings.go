package vaultfs

import "strings"

func normalizeStrings(source []string) []string {
	normalizedArr := make([]string, 0, len(source))

	for _, val := range source {
		normalizedVal := strings.ToLower(strings.TrimSpace(val))
		if normalizedVal != "" {
			normalizedArr = append(normalizedArr, normalizedVal)
		}
	}

	return normalizedArr
}
