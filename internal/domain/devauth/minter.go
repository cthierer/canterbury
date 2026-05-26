package devauth

import (
	"context"
	"time"
)

// MintOptions contains caller-controlled token minting options.
type MintOptions struct {
	TTL time.Duration
}

// Minter signs development tokens and exposes their verification key.
type Minter interface {
	MintToken(ctx context.Context, claims Claims, issuedAt time.Time, expiresAt time.Time) (Token, error)
	VerificationKey() VerificationKey
}
