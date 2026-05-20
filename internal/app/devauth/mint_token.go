package devauth

import (
	"context"
	"fmt"
	"strings"
	"time"

	domain "github.com/cthierer/canterbury/internal/domain/devauth"
)

const (
	maxTTL     = 1 * time.Hour
	defaultTTL = 15 * time.Minute
)

// MintToken validates a development token request and delegates signing.
func (service *Service) MintToken(ctx context.Context, claims domain.Claims, options domain.MintOptions) (domain.Token, error) {
	if err := ctx.Err(); err != nil {
		return domain.Token{}, err
	}

	subject := strings.TrimSpace(claims.Subject)
	if subject == "" {
		return domain.Token{}, domain.ErrMissingSubject
	}

	audiences := formatAudiences(claims.Audiences)
	if len(audiences) < 1 || len(audiences) != len(claims.Audiences) {
		return domain.Token{}, domain.ErrMissingAudience
	}

	issuedAt := service.clock.Now()
	expiresAt, err := expiryForTTL(issuedAt, options.TTL)
	if err != nil {
		return domain.Token{}, fmt.Errorf("apply options: %w", err)
	}

	token, err := service.minter.MintToken(ctx, domain.Claims{
		Subject:   subject,
		Audiences: audiences,
	}, issuedAt, expiresAt)
	if err != nil {
		return domain.Token{}, fmt.Errorf("mint token: %w", err)
	}

	return token, nil
}

func expiryForTTL(issuedAt time.Time, ttl time.Duration) (time.Time, error) {
	if ttl < 0 {
		return time.Time{}, domain.ErrNegativeTTL
	}

	if ttl > maxTTL {
		return time.Time{}, domain.ErrLargeTTL
	}

	if ttl == 0 {
		ttl = defaultTTL
	}

	return issuedAt.Add(ttl), nil
}

func formatAudiences(source []string) []string {
	audiences := make([]string, 0, len(source))

	for _, audience := range source {
		trimmed := strings.TrimSpace(audience)
		if trimmed == "" {
			continue
		}

		audiences = append(audiences, trimmed)
	}

	return audiences
}
