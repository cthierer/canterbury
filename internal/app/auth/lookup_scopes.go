package auth

import (
	"context"

	"github.com/cthierer/canterbury/internal/domain/vault"
)

// ScopeLookup describes the scopes resolved for a verified principal.
type ScopeLookup struct {
	Scopes          []vault.Scope
	MappingChecksum string
}

// LookupScopes returns mapped scopes for an issuer and subject pair.
func (mapper *ScopeMapper) LookupScopes(ctx context.Context, issuer string, subject string) (ScopeLookup, error) {
	if err := ctx.Err(); err != nil {
		return ScopeLookup{}, err
	}

	result := ScopeLookup{MappingChecksum: mapper.sourceChecksum}

	key := subjectKey{issuer: issuer, subject: subject}
	scopes, exists := mapper.lookup[key]
	if !exists {
		return result, nil
	}

	result.Scopes = append([]vault.Scope(nil), scopes...)

	return result, nil
}
