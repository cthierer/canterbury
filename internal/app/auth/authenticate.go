package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// Authenticate verifies token and resolves the resulting caller principal.
func (authenticator *Authenticator) Authenticate(ctx context.Context, token string) (Principal, error) {
	if err := ctx.Err(); err != nil {
		return Principal{}, err
	}

	claims, err := authenticator.verifier.VerifyToken(ctx, token, authenticator.tokenRequirements())
	if err != nil {
		return Principal{}, fmt.Errorf("verify token: %w", err)
	}

	issuer := strings.TrimSpace(claims.Issuer)
	if issuer == "" {
		return Principal{}, fmt.Errorf("issuer is required")
	}

	subject := strings.TrimSpace(claims.Subject)
	if subject == "" {
		return Principal{}, withFailureContext(ErrMissingSubject, FailureContext{
			Issuer: issuer,
		})
	}

	scopes, err := authenticator.scopeMapper.LookupScopes(ctx, issuer, subject)
	if err != nil {
		return Principal{}, fmt.Errorf("lookup scopes: %w", withFailureContext(err, FailureContext{
			Issuer:      issuer,
			SubjectHash: hashSubject(issuer, subject),
		}))
	}
	if len(scopes.Scopes) == 0 {
		return Principal{}, withFailureContext(ErrPrincipalResolutionFailed, FailureContext{
			Issuer:      issuer,
			SubjectHash: hashSubject(issuer, subject),
		})
	}

	return Principal{
		Issuer:          issuer,
		Subject:         subject,
		SubjectHash:     hashSubject(issuer, subject),
		Scopes:          scopes.Scopes,
		MappingChecksum: scopes.MappingChecksum,
	}, nil
}

func (authenticator *Authenticator) tokenRequirements() TokenRequirements {
	return TokenRequirements{
		Issuer:   authenticator.issuer,
		Audience: authenticator.audience,
	}
}

func hashSubject(issuer string, subject string) string {
	sum := sha256.Sum256([]byte(issuer + "\x00" + subject))
	return "sha256:" + hex.EncodeToString(sum[:])
}
