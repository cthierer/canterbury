package auth

import domain "github.com/cthierer/canterbury/internal/domain/vault"

// Principal describes the caller identity relevant to application policy.
type Principal struct {
	Scopes []domain.Scope
}
