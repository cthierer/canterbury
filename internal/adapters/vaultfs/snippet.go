package vaultfs

import (
	"strings"
	"unicode/utf8"

	"github.com/cthierer/canterbury/internal/domain/vault"
)

const defaultSnippetLength = 240
const snippetEllipsis = "..."

func buildSnippet(note vault.Note, search vault.TextSearch) string {
	content := normalizeSnippetText(note.Content)
	if content == "" {
		return ""
	}

	start := 0
	if matchIndex := firstTermIndex(content, search.Terms); matchIndex >= 0 {
		start = snippetStart(content, matchIndex, defaultSnippetLength)
	}

	return truncateSnippet(content, start, defaultSnippetLength)
}

func normalizeSnippetText(content string) string {
	return strings.Join(strings.Fields(content), " ")
}

func firstTermIndex(content string, terms []string) int {
	normalizedTerms := normalizeStrings(terms)
	if len(normalizedTerms) == 0 {
		return -1
	}

	lowerContent := strings.ToLower(content)
	firstIndex := -1
	for _, term := range normalizedTerms {
		index := strings.Index(lowerContent, term)
		if index >= 0 && (firstIndex == -1 || index < firstIndex) {
			firstIndex = index
		}
	}

	return firstIndex
}

func snippetStart(content string, matchIndex int, limit int) int {
	if matchIndex <= limit/2 {
		return 0
	}

	return previousRuneBoundary(content, matchIndex-limit/2)
}

func truncateSnippet(content string, start int, limit int) string {
	if limit <= 0 {
		return ""
	}

	start = clampToRuneBoundary(content, start)
	if start >= len(content) {
		return ""
	}

	end := start
	for end < len(content) && utf8.RuneCountInString(content[start:end]) < limit {
		_, size := utf8.DecodeRuneInString(content[end:])
		end += size
	}

	snippet := content[start:end]
	if start > 0 {
		snippet = snippetEllipsis + snippet
	}

	if end < len(content) {
		snippet += snippetEllipsis
	}

	return snippet
}

func clampToRuneBoundary(content string, index int) int {
	if index <= 0 {
		return 0
	}

	if index >= len(content) {
		return len(content)
	}

	return previousRuneBoundary(content, index)
}

func previousRuneBoundary(content string, index int) int {
	for index > 0 && !utf8.RuneStart(content[index]) {
		index--
	}

	return index
}
