package devauth

import (
	"fmt"
	"strings"
	"time"

	domain "github.com/cthierer/canterbury/internal/domain/devauth"
)

const (
	maxTTL     = 1 * time.Hour
	defaultTTL = 15 * time.Minute
)

// MintOption customizes a single token minting request.
type MintOption func(domain.MintOptions) (domain.MintOptions, error)

// MintToken validates a development token request and delegates signing.
func (service *Service) MintToken(claims domain.Claims, options ...MintOption) (domain.Token, error) {
	subject := strings.TrimSpace(claims.Subject)
	if subject == "" {
		return domain.Token{}, ErrMissingSubject
	}

	audiences := formatAudiences(claims.Audiences)
	if len(audiences) < 1 || len(audiences) != len(claims.Audiences) {
		return domain.Token{}, ErrMissingAudience
	}

	issuedAt := service.clock.Now()
	mintOptions, err := applyOptions(domain.MintOptions{
		IssuedAt:  issuedAt,
		ExpiresAt: issuedAt.Add(defaultTTL),
	}, options)
	if err != nil {
		return domain.Token{}, fmt.Errorf("apply options: %w", err)
	}

	token, err := service.minter.MintToken(domain.Claims{
		Subject:   subject,
		Audiences: audiences,
	}, mintOptions)
	if err != nil {
		return domain.Token{}, fmt.Errorf("mint token: %w", err)
	}

	return token, nil
}

// WithTTL sets the lifetime for a minted token.
func WithTTL(ttl time.Duration) MintOption {
	return func(options domain.MintOptions) (domain.MintOptions, error) {
		if ttl < 0 {
			return domain.MintOptions{}, ErrNegativeTTL
		}

		if ttl > maxTTL {
			return domain.MintOptions{}, ErrLargeTTL
		}

		if ttl == 0 {
			ttl = defaultTTL
		}

		options.ExpiresAt = options.IssuedAt.Add(ttl)
		return options, nil
	}
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

func applyOptions(to domain.MintOptions, options []MintOption) (domain.MintOptions, error) {
	curr := to
	for _, option := range options {
		next, err := option(curr)
		if err != nil {
			return domain.MintOptions{}, fmt.Errorf("apply option: %w", err)
		}

		curr = next
	}

	return curr, nil
}
