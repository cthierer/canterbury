package devauth

import "time"

// TokenType names the kind of token returned to development clients.
type TokenType string

func (t TokenType) String() string {
	return string(t)
}

// TokenTypeBearer indicates a bearer token suitable for Authorization headers.
const TokenTypeBearer TokenType = "Bearer"

// Token contains a minted development token and its expiry.
type Token struct {
	JWT       string
	Type      TokenType
	ExpiresAt time.Time
}
