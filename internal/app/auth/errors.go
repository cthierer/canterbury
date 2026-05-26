package auth

import "errors"

var (
	// ErrMalformedToken indicates a bearer token could not be parsed as a JWT.
	ErrMalformedToken = errors.New("malformed token")

	// ErrInvalidTokenSignature indicates a JWT signature or signing method was invalid.
	ErrInvalidTokenSignature = errors.New("invalid token signature")

	// ErrExpiredToken indicates a JWT is expired.
	ErrExpiredToken = errors.New("expired token")

	// ErrWrongIssuer indicates a JWT issuer did not match Canterbury auth policy.
	ErrWrongIssuer = errors.New("wrong issuer")

	// ErrWrongAudience indicates a JWT audience did not match Canterbury auth policy.
	ErrWrongAudience = errors.New("wrong audience")

	// ErrMissingSubject indicates a verified token did not contain a usable subject.
	ErrMissingSubject = errors.New("missing subject")

	// ErrPrincipalResolutionFailed indicates identity could not be resolved into a principal.
	ErrPrincipalResolutionFailed = errors.New("principal resolution failed")
)

// FailureContext carries safe audit context for authentication failures.
type FailureContext struct {
	Issuer      string
	SubjectHash string
}

type failureContextError struct {
	cause   error
	context FailureContext
}

func (err failureContextError) Error() string {
	return err.cause.Error()
}

func (err failureContextError) Unwrap() error {
	return err.cause
}

func (err failureContextError) FailureContext() FailureContext {
	return err.context
}

func withFailureContext(cause error, context FailureContext) error {
	return failureContextError{
		cause:   cause,
		context: context,
	}
}

// FailureContextFromError returns safe audit context carried by an auth error.
func FailureContextFromError(err error) (FailureContext, bool) {
	var contextual interface {
		FailureContext() FailureContext
	}

	if !errors.As(err, &contextual) {
		return FailureContext{}, false
	}

	return contextual.FailureContext(), true
}
