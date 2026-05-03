package auditfs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cthierer/canterbury/internal/domain/audit"
)

var _ audit.Recorder = (*Recorder)(nil)

// Recorder stores audit events in date-rotated JSON Lines files.
type Recorder struct {
	// root is trusted to be valid and sanitized, at least at initialization.
	root string
}

// NewRecorder creates a filesystem audit recorder rooted at root.
//
// The root directory is created if needed and resolved to an absolute path before
// records are written below date-based subdirectories.
func NewRecorder(root string) (*Recorder, error) {
	root = strings.TrimSpace(root)

	if root == "" {
		return nil, fmt.Errorf("create audit log filesystem repository: %w", ErrInvalidRoot)
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve audit log root %q absolute path: %w", root, err)
	}

	err = os.MkdirAll(absRoot, 0o700)
	if err != nil {
		return nil, fmt.Errorf("create audit log root: %w", err)
	}

	resolvedRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve audit log root %q symlinks: %w", root, err)
	}

	return &Recorder{root: resolvedRoot}, nil
}
