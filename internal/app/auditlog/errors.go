package auditlog

import "errors"

// ErrInvalidTimestamp indicates a timestamp cannot be encoded in an audit event
// identifier.
var ErrInvalidTimestamp = errors.New("invalid timestamp")
