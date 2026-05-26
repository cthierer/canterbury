package authfs

import "fmt"

var (
	// ErrInvalidFilePath indicates the mapping file path is empty.
	ErrInvalidFilePath = fmt.Errorf("invalid file path")

	// ErrUnrecognizedKeys indicates the mapping file contains unsupported TOML keys.
	ErrUnrecognizedKeys = fmt.Errorf("unrecognized keys in TOML file")
)
