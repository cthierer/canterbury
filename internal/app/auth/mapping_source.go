package auth

import (
	"context"

	"github.com/cthierer/canterbury/internal/domain/vault"
)

// MappingSource loads principal-to-scope mapping data for application auth.
type MappingSource interface {
	LoadMapping(ctx context.Context) (MappingDocument, error)
}

// MappingDocument contains validated scope mapping entries from one policy source.
type MappingDocument struct {
	Checksum string
	Subjects []MappingSubject
}

// MappingSubject maps one issuer and subject pair to Canterbury vault scopes.
type MappingSubject struct {
	Issuer  string
	Subject string
	Scopes  []vault.Scope
}
