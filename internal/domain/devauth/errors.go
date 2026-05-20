package devauth

import "errors"

var (
	// ErrMissingSubject indicates a token request did not include a subject.
	ErrMissingSubject = errors.New("subject must be present")
	// ErrMissingAudience indicates a token request did not include any audience.
	ErrMissingAudience = errors.New("audience must be present")
	// ErrNegativeTTL indicates a token request used a negative lifetime.
	ErrNegativeTTL = errors.New("ttl must not be negative")
	// ErrLargeTTL indicates a token request exceeded the maximum lifetime.
	ErrLargeTTL = errors.New("ttl must be within range")
)
