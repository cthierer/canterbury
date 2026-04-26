package vault

import (
	"path"
	"strings"
	"time"
)

// NotePath identifies a Markdown note by normalized vault-relative path.
type NotePath string

// Tag identifies an Obsidian-style note tag.
type Tag string

// NoteRef is a lightweight reference to a note.
type NoteRef struct {
	Path  NotePath
	Title string
}

// Note contains full note content and parsed metadata.
type Note struct {
	Ref      NoteRef
	Metadata NoteMetadata
	Content  string
}

// NoteMetadata contains structured metadata parsed from a note and its file.
type NoteMetadata struct {
	Path           NotePath
	Title          string
	Tags           []Tag
	Access         ResourceAccess
	Frontmatter    map[string]any
	SizeBytes      int64
	ModifiedAt     time.Time
	HasFrontmatter bool
}

// NewNotePath validates and normalizes a vault-relative Markdown path.
func NewNotePath(value string) (NotePath, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return "", ErrInvalidNotePath
	}

	if strings.Contains(normalized, "\\") {
		return "", ErrInvalidNotePath
	}

	if strings.HasPrefix(normalized, "/") {
		return "", ErrInvalidNotePath
	}

	cleaned := path.Clean(normalized)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", ErrInvalidNotePath
	}

	if !strings.HasSuffix(strings.ToLower(cleaned), ".md") {
		return "", ErrInvalidNotePath
	}

	return NotePath(cleaned), nil
}

func (p NotePath) String() string {
	return string(p)
}
