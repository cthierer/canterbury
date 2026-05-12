package devauthjwt

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/cthierer/canterbury/internal/domain/devauth"
)

var _ devauth.Minter = (*Minter)(nil)

// Minter signs development JWTs with an in-memory Ed25519 key pair.
type Minter struct {
	issuer     string
	keyID      string
	publicKey  ed25519.PublicKey
	privateKey ed25519.PrivateKey
}

// NewMinter creates a JWT minter for the given issuer.
func NewMinter(issuer string) (*Minter, error) {
	issuer = strings.TrimSpace(issuer)
	if issuer == "" {
		return nil, ErrInvalidIssuer
	}

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate ed25519 key: %w", err)
	}

	sum := sha256.Sum256(publicKey)
	keyID := base64.RawURLEncoding.EncodeToString(sum[:])

	minter := Minter{
		issuer:     issuer,
		keyID:      keyID,
		publicKey:  publicKey,
		privateKey: privateKey,
	}

	return &minter, nil
}
