package auth

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// TokenClaims contains trusted identity claims from a verified token.
type TokenClaims struct {
	Issuer    string
	Subject   string
	Audiences []string
	ExpiresAt time.Time
}

// TokenRequirements defines Canterbury auth policy checks for token validation.
type TokenRequirements struct {
	Issuer   string
	Audience string
}

// TokenVerifier validates signed tokens and returns trusted identity claims.
type TokenVerifier interface {
	VerifyToken(ctx context.Context, token string, requirements TokenRequirements) (TokenClaims, error)
}

// Authenticator resolves verified tokens into Canterbury principals.
type Authenticator struct {
	issuer      string
	audience    string
	scopeMapper *ScopeMapper
	verifier    TokenVerifier
}

// NewAuthenticator creates an application auth service.
func NewAuthenticator(issuer string, audience string, scopeMapper *ScopeMapper, verifier TokenVerifier) (*Authenticator, error) {
	issuer = strings.TrimSpace(issuer)
	if issuer == "" {
		return nil, fmt.Errorf("issuer is required")
	}

	audience = strings.TrimSpace(audience)
	if audience == "" {
		return nil, fmt.Errorf("audience is required")
	}

	if scopeMapper == nil {
		return nil, fmt.Errorf("scope mapper is required")
	}

	if verifier == nil {
		return nil, fmt.Errorf("verifier is required")
	}

	return &Authenticator{
		issuer:      issuer,
		audience:    audience,
		scopeMapper: scopeMapper,
		verifier:    verifier,
	}, nil
}

// MappingChecksum returns the checksum for the scope mapping used by authenticator.
func (authenticator *Authenticator) MappingChecksum() string {
	return authenticator.scopeMapper.MappingChecksum()
}
