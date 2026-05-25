package auth

import domain "github.com/cthierer/canterbury/internal/domain/vault"

// Principal describes the caller identity relevant to application policy.
type Principal struct {
	Issuer          string
	Subject         string
	SubjectHash     string
	Scopes          []domain.Scope
	MappingChecksum string
}
