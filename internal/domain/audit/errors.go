package audit

import "errors"

var (
	// ErrInvalidEventID indicates an event ID is empty or too long.
	ErrInvalidEventID = errors.New("invalid event ID")

	// ErrInvalidRequestID indicates a request ID is empty or too long.
	ErrInvalidRequestID = errors.New("invalid request ID")

	// ErrInvalidTraceID indicates a trace ID is too long.
	ErrInvalidTraceID = errors.New("invalid trace ID")
)
