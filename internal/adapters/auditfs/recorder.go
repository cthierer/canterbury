package auditfs

import (
	"crypto/rand"
	"encoding/hex"
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
	root     string
	writerID string
}

type recorderConfig struct {
	writerID string
}

// RecorderOption configures a filesystem audit recorder.
type RecorderOption func(*recorderConfig) error

// WithWriterID sets the identifier included in audit log filenames.
func WithWriterID(id string) RecorderOption {
	return func(config *recorderConfig) error {
		writerID, err := normalizeExplicitWriterID(id)
		if err != nil {
			return err
		}

		config.writerID = writerID
		return nil
	}
}

// NewRecorder creates a filesystem audit recorder rooted at root.
//
// The root directory is created if needed and resolved to an absolute path before
// records are written below date-based subdirectories.
func NewRecorder(root string, opts ...RecorderOption) (*Recorder, error) {
	root = strings.TrimSpace(root)

	if root == "" {
		return nil, ErrInvalidRoot
	}

	config := recorderConfig{}
	for _, opt := range opts {
		if err := opt(&config); err != nil {
			return nil, fmt.Errorf("configure audit log filesystem recorder: %w", err)
		}
	}

	writerID := config.writerID
	if writerID == "" {
		generatedWriterID, err := generateWriterID()
		if err != nil {
			return nil, fmt.Errorf("generate audit log writer ID: %w", err)
		}

		writerID = generatedWriterID
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

	return &Recorder{
		root:     resolvedRoot,
		writerID: writerID,
	}, nil
}

func generateWriterID() (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "host"
	}

	hostname = sanitizeWriterIDComponent(hostname)
	if hostname == "" {
		hostname = "host"
	}

	randomBytes := make([]byte, 4)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("read random writer ID suffix: %w", err)
	}

	writerID := fmt.Sprintf(
		"%s-pid%d-%s",
		hostname,
		os.Getpid(),
		hex.EncodeToString(randomBytes),
	)

	return writerID, nil
}

func normalizeExplicitWriterID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", ErrInvalidWriterID
	}

	for _, char := range id {
		if !isWriterIDChar(char) {
			return "", ErrInvalidWriterID
		}
	}

	return id, nil
}

func sanitizeWriterIDComponent(value string) string {
	var builder strings.Builder
	lastWasSeparator := false

	for _, char := range strings.TrimSpace(value) {
		if isWriterIDChar(char) {
			builder.WriteRune(char)
			lastWasSeparator = false
			continue
		}

		if !lastWasSeparator {
			builder.WriteByte('-')
			lastWasSeparator = true
		}
	}

	return strings.Trim(builder.String(), ".-_")
}

func isWriterIDChar(char rune) bool {
	return char >= 'A' && char <= 'Z' ||
		char >= 'a' && char <= 'z' ||
		char >= '0' && char <= '9' ||
		char == '.' ||
		char == '_' ||
		char == '-'
}
