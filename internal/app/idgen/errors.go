package idgen

import "errors"

// ErrInvalidTimestamp indicates a timestamp cannot be encoded in an identifier.
var ErrInvalidTimestamp = errors.New("invalid timestamp")
