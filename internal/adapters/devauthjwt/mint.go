package devauthjwt

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cthierer/canterbury/internal/domain/devauth"
)

type header struct {
	Type      string `json:"typ"`
	Algorithm string `json:"alg"`
	KeyID     string `json:"kid"`
}

type payload struct {
	Issuer    string   `json:"iss"`
	Subject   string   `json:"sub"`
	Audiences []string `json:"aud"`
	ExpiresAt int64    `json:"exp"`
	IssuedAt  int64    `json:"iat"`
}

// MintToken builds and signs a JWT for validated development auth claims.
func (minter *Minter) MintToken(ctx context.Context, claims devauth.Claims, issuedAt time.Time, expiresAt time.Time) (devauth.Token, error) {
	if err := ctx.Err(); err != nil {
		return devauth.Token{}, err
	}

	header := header{
		Type:      "JWT",
		Algorithm: "EdDSA",
		KeyID:     minter.keyID,
	}

	payload := payload{
		Issuer:    minter.issuer,
		Subject:   claims.Subject,
		Audiences: claims.Audiences,
		ExpiresAt: expiresAt.Unix(),
		IssuedAt:  issuedAt.Unix(),
	}

	jwt, err := minter.buildJWT(header, payload)
	if err != nil {
		return devauth.Token{}, fmt.Errorf("build JWT: %w", err)
	}

	token := devauth.Token{
		JWT:       jwt,
		Type:      devauth.TokenTypeBearer,
		ExpiresAt: expiresAt,
	}

	return token, nil
}

func (minter Minter) buildJWT(header header, payload payload) (string, error) {
	headerVal, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("marshal header: %w", err)
	}

	payloadVal, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}

	signingInput := fmt.Sprintf(
		"%s.%s",
		base64.RawURLEncoding.EncodeToString(headerVal),
		base64.RawURLEncoding.EncodeToString(payloadVal),
	)

	signature := ed25519.Sign(minter.privateKey, []byte(signingInput))

	return fmt.Sprintf(
		"%s.%s",
		signingInput,
		base64.RawURLEncoding.EncodeToString(signature),
	), nil
}
