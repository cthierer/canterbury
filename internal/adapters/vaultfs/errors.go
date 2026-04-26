package vaultfs

import "errors"

var (
	// ErrInvalidRoot indicates the configured vault root is empty or invalid.
	ErrInvalidRoot = errors.New("invalid vault root")

	// ErrFileTooLarge indicates a note exceeds the adapter's supported read size.
	ErrFileTooLarge = errors.New("file exceeds maximum supported file size")

	// ErrIsDirectory indicates a resolved note path points to a directory.
	ErrIsDirectory = errors.New("cannot read directory as note")
)
