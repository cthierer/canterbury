package vaultfs

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cthierer/canterbury/internal/domain/vault"
)

var _ vault.Repository = (*Repository)(nil)

// Repository implements vault.Repository against a local filesystem vault mirror.
type Repository struct {
	// root is trusted to be valid and sanitized, at least at initialization.
	root string
}

// NewRepository creates a filesystem-backed vault repository rooted at root.
func NewRepository(root string) (*Repository, error) {
	root = strings.TrimSpace(root)

	if root == "" {
		return nil, fmt.Errorf("create vault filesystem repository: %w", ErrInvalidRoot)
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve vault root %q absolute path: %w", root, err)
	}

	resolvedRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve vault root %q symlinks: %w", root, err)
	}

	return &Repository{root: resolvedRoot}, nil
}
