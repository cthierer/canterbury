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
