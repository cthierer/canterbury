package auditfs

import "errors"

var (
	// ErrInvalidRoot indicates the audit log root is empty or otherwise invalid.
	ErrInvalidRoot = errors.New("invalid audit log root")

	// ErrInvalidWriterID indicates the audit log writer ID is empty or unsafe.
	ErrInvalidWriterID = errors.New("invalid audit log writer ID")

	// ErrEventMissingID indicates an event cannot be recorded without an ID.
	ErrEventMissingID = errors.New("event must have an ID")

	// ErrEventUnknownType indicates an event has no recognized audit event type.
	ErrEventUnknownType = errors.New("event must have a recognized type")

	// ErrInvalidScopes indicates an event contains an empty or invalid scope.
	ErrInvalidScopes = errors.New("invalid scopes provided")

	// ErrEventMissingTimestamp indicates an event cannot be recorded without a
	// timestamp.
	ErrEventMissingTimestamp = errors.New("event must have a valid timestamp")
)
