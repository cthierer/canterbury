package vault

import "strings"

// Scope names a permission boundary that can be granted to a principal and
// declared by a vault resource.
type Scope string

// ResourceAccess describes the controlled service exposure declared by a note.
// Missing or empty scopes are interpreted by policy layers as default deny.
type ResourceAccess struct {
	Scopes []Scope
}

// NewScope validates and normalizes a scope value.
func NewScope(value string) (Scope, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return "", ErrInvalidScope
	}

	return Scope(normalized), nil
}

func (s Scope) String() string {
	return string(s)
}

// HasScope reports whether the resource declares the given scope.
func (a ResourceAccess) HasScope(scope Scope) bool {
	for _, allowedScope := range a.Scopes {
		if allowedScope == scope {
			return true
		}
	}

	return false
}

// AllowsAny reports whether the resource declares any of the given scopes.
func (a ResourceAccess) AllowsAny(scopes []Scope) bool {
	for _, scope := range scopes {
		if a.HasScope(scope) {
			return true
		}
	}

	return false
}
