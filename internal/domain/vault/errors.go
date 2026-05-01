package vault

import "errors"

var (
	// ErrInvalidNotePath indicates a note path is empty, unsafe, or unsupported.
	ErrInvalidNotePath = errors.New("invalid note path")

	// ErrInvalidTag indicates a tag is empty or not supported.
	ErrInvalidTag = errors.New("invalid tag")

	// ErrInvalidScope indicates a scope value is empty or malformed.
	ErrInvalidScope = errors.New("invalid scope")

	// ErrInvalidSearch indicates a search query cannot be executed.
	ErrInvalidSearch = errors.New("invalid search query")

	// ErrNoteNotFound indicates the requested note does not exist.
	ErrNoteNotFound = errors.New("note not found")

	// ErrVaultUnavailable indicates the backing vault cannot be accessed.
	ErrVaultUnavailable = errors.New("vault unavailable")
)
