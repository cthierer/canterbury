package vaultfs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cthierer/canterbury/internal/domain/vault"
)

func joinVaultPath(resolvedRoot string, notePath vault.NotePath) (string, error) {
	candidate := filepath.Join(resolvedRoot, filepath.FromSlash(notePath.String()))

	resolvedCandidate, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("resolve note path %q: %w", notePath, vault.ErrNoteNotFound)
		}

		return "", fmt.Errorf("resolve note path %q symlinks: %w", notePath, err)
	}

	if !isWithinRoot(resolvedRoot, resolvedCandidate) {
		return "", fmt.Errorf("resolve note path %q: %w", notePath, vault.ErrInvalidNotePath)
	}

	return resolvedCandidate, nil
}

func isWithinRoot(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}

	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
