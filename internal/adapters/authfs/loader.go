package authfs

import (
	"strings"
)

// Loader reads scope mapping data from a TOML file on disk.
type Loader struct {
	filePath string
}

// NewLoader creates a filesystem mapping loader for the given TOML file path.
func NewLoader(filePath string) (*Loader, error) {
	filePath = strings.TrimSpace(filePath)

	if filePath == "" {
		return nil, ErrInvalidFilePath
	}

	return &Loader{filePath: filePath}, nil
}
