package auth

import (
	"context"
	"fmt"
	"strings"

	"github.com/cthierer/canterbury/internal/domain/vault"
)

// ScopeMapper resolves verified issuer and subject pairs to Canterbury scopes.
type ScopeMapper struct {
	sourceChecksum string
	lookup         map[subjectKey][]vault.Scope
}

// NewScopeMapper loads and validates scope mappings from source.
func NewScopeMapper(ctx context.Context, source MappingSource) (*ScopeMapper, error) {
	if source == nil {
		return nil, fmt.Errorf("mapping source is required")
	}

	document, err := source.LoadMapping(ctx)
	if err != nil {
		return nil, fmt.Errorf("load mapping: %w", err)
	}

	lookup := make(map[subjectKey][]vault.Scope)
	for _, mapping := range document.Subjects {
		issuer := strings.TrimSpace(mapping.Issuer)
		if issuer == "" {
			return nil, fmt.Errorf("issuer must not be blank")
		}

		subject := strings.TrimSpace(mapping.Subject)
		if subject == "" {
			return nil, fmt.Errorf("subject must not be blank")
		}

		key := subjectKey{issuer: issuer, subject: subject}
		if _, exists := lookup[key]; exists {
			return nil, fmt.Errorf("duplicate scope mapping for issuer %q, subject %q", issuer, subject)
		}

		lookup[key] = append([]vault.Scope(nil), mapping.Scopes...)
	}

	return &ScopeMapper{
		sourceChecksum: document.Checksum,
		lookup:         lookup,
	}, nil
}

// MappingChecksum returns the checksum for the loaded mapping source.
func (mapper *ScopeMapper) MappingChecksum() string {
	return mapper.sourceChecksum
}

type subjectKey struct {
	issuer  string
	subject string
}
