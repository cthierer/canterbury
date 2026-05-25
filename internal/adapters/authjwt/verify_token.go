package authjwt

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cthierer/canterbury/internal/app/auth"
	"github.com/golang-jwt/jwt/v5"
)

// VerifyToken validates token against the configured keys and token requirements.
func (verifier *Verifier) VerifyToken(ctx context.Context, token string, requirements auth.TokenRequirements) (auth.TokenClaims, error) {
	if err := ctx.Err(); err != nil {
		return auth.TokenClaims{}, err
	}

	opts := []jwt.ParserOption{
		jwt.WithValidMethods(verifier.methods),
		jwt.WithTimeFunc(verifier.clock.Now),
		jwt.WithExpirationRequired(),
	}

	issuer := strings.TrimSpace(requirements.Issuer)
	if len(issuer) > 0 {
		opts = append(opts, jwt.WithIssuer(issuer))
	}

	audience := strings.TrimSpace(requirements.Audience)
	if len(audience) > 0 {
		opts = append(opts, jwt.WithAudience(audience))
	}

	parsed, err := jwt.Parse(token, verifier.keyFunc, opts...)
	if err != nil {
		return auth.TokenClaims{}, classifyTokenError(err)
	}

	var claims auth.TokenClaims

	claims.Issuer, err = parsed.Claims.GetIssuer()
	if err != nil {
		return auth.TokenClaims{}, fmt.Errorf("extract issuer: %w", err)
	}

	claims.Subject, err = parsed.Claims.GetSubject()
	if err != nil {
		return auth.TokenClaims{}, fmt.Errorf("extract subject: %w", err)
	}

	audiences, err := parsed.Claims.GetAudience()
	if err != nil {
		return auth.TokenClaims{}, fmt.Errorf("extract audience: %w", err)
	}

	claims.Audiences = append([]string(nil), audiences...)

	expiration, err := parsed.Claims.GetExpirationTime()
	if err != nil {
		return auth.TokenClaims{}, fmt.Errorf("extract expiration: %w", err)
	}

	if expiration != nil {
		claims.ExpiresAt = expiration.Time
	}

	return claims, nil
}

func classifyTokenError(err error) error {
	switch {
	case errors.Is(err, jwt.ErrTokenMalformed):
		return auth.ErrMalformedToken
	case errors.Is(err, jwt.ErrTokenSignatureInvalid):
		return auth.ErrInvalidTokenSignature
	case errors.Is(err, jwt.ErrTokenExpired):
		return auth.ErrExpiredToken
	case errors.Is(err, jwt.ErrTokenInvalidIssuer):
		return auth.ErrWrongIssuer
	case errors.Is(err, jwt.ErrTokenInvalidAudience):
		return auth.ErrWrongAudience
	case errors.Is(err, jwt.ErrTokenUnverifiable):
		return auth.ErrPrincipalResolutionFailed
	default:
		return auth.ErrMalformedToken
	}
}
