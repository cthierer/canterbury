package devauth

import "time"

// MintOptions contains service-controlled token timing values.
type MintOptions struct {
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// Minter signs development tokens and exposes their verification key.
type Minter interface {
	MintToken(claims Claims, options MintOptions) (Token, error)
	VerificationKey() VerificationKey
}
