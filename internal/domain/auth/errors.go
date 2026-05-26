// Package auth defines authorization errors shared by application boundaries.
package auth

import "errors"

var (
	// ErrPermissionDenied indicates the principal may not access a vault resource.
	ErrPermissionDenied = errors.New("permission denied")
	// ErrMissingPrincipal indicates the principal is not set.
	ErrMissingPrincipal = errors.New("missing principal")
)
