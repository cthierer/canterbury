package devauthjwt

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/cthierer/canterbury/internal/domain/devauth"
)

func TestNewMinterRejectsMissingIssuer(t *testing.T) {
	_, err := NewMinter(" ")
	if !errors.Is(err, ErrInvalidIssuer) {
		t.Fatalf("NewMinter() error = %v, want %v", err, ErrInvalidIssuer)
	}
}

func TestVerificationKeyMatchesGeneratedPublicKey(t *testing.T) {
	minter := newTestMinter(t)

	key := minter.VerificationKey()
	if key.Algorithm != devauth.SigningAlgorithmEdDSA {
		t.Fatalf("algorithm = %q, want %q", key.Algorithm, devauth.SigningAlgorithmEdDSA)
	}

	if !reflect.DeepEqual(key.PublicKey, minter.publicKey) {
		t.Fatalf("public key does not match generated minter key")
	}

	sum := sha256.Sum256(minter.publicKey)
	wantID := base64.RawURLEncoding.EncodeToString(sum[:])
	if key.ID != wantID {
		t.Fatalf("key ID = %q, want %q", key.ID, wantID)
	}
}

func TestMintTokenBuildsSignedJWT(t *testing.T) {
	minter := newTestMinter(t)
	issuedAt := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
	expiresAt := issuedAt.Add(15 * time.Minute)

	token, err := minter.MintToken(
		devauth.Claims{
			Subject:   "user_123",
			Audiences: []string{"https://canterbury.example.test"},
		},
		devauth.MintOptions{
			IssuedAt:  issuedAt,
			ExpiresAt: expiresAt,
		},
	)
	if err != nil {
		t.Fatalf("MintToken() error = %v", err)
	}

	if token.Type != devauth.TokenTypeBearer {
		t.Fatalf("token type = %q, want %q", token.Type, devauth.TokenTypeBearer)
	}
	if token.ExpiresAt != expiresAt {
		t.Fatalf("expires at = %v, want %v", token.ExpiresAt, expiresAt)
	}

	parts := strings.Split(token.JWT, ".")
	if len(parts) != 3 {
		t.Fatalf("JWT part count = %d, want 3", len(parts))
	}

	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatalf("decode signature: %v", err)
	}
	signingInput := parts[0] + "." + parts[1]
	if !ed25519.Verify(minter.publicKey, []byte(signingInput), signature) {
		t.Fatalf("JWT signature did not verify")
	}

	var gotHeader header
	decodeJWTPart(t, parts[0], &gotHeader)
	if gotHeader.Type != "JWT" {
		t.Fatalf("header typ = %q, want %q", gotHeader.Type, "JWT")
	}
	if gotHeader.Algorithm != "EdDSA" {
		t.Fatalf("header alg = %q, want %q", gotHeader.Algorithm, "EdDSA")
	}
	if gotHeader.KeyID != minter.keyID {
		t.Fatalf("header kid = %q, want %q", gotHeader.KeyID, minter.keyID)
	}

	var gotPayload payload
	decodeJWTPart(t, parts[1], &gotPayload)
	if gotPayload.Issuer != "https://dev-auth.example.test" {
		t.Fatalf("issuer = %q, want %q", gotPayload.Issuer, "https://dev-auth.example.test")
	}
	if gotPayload.Subject != "user_123" {
		t.Fatalf("subject = %q, want %q", gotPayload.Subject, "user_123")
	}
	if !reflect.DeepEqual(gotPayload.Audiences, []string{"https://canterbury.example.test"}) {
		t.Fatalf("audiences = %#v", gotPayload.Audiences)
	}
	if gotPayload.IssuedAt != issuedAt.Unix() {
		t.Fatalf("iat = %d, want %d", gotPayload.IssuedAt, issuedAt.Unix())
	}
	if gotPayload.ExpiresAt != expiresAt.Unix() {
		t.Fatalf("exp = %d, want %d", gotPayload.ExpiresAt, expiresAt.Unix())
	}
}

func newTestMinter(t *testing.T) *Minter {
	t.Helper()

	minter, err := NewMinter(" https://dev-auth.example.test ")
	if err != nil {
		t.Fatalf("NewMinter() error = %v", err)
	}

	return minter
}

func decodeJWTPart(t *testing.T, value string, target any) {
	t.Helper()

	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		t.Fatalf("decode JWT part: %v", err)
	}

	if err := json.Unmarshal(decoded, target); err != nil {
		t.Fatalf("unmarshal JWT part: %v", err)
	}
}
