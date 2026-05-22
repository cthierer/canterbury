package keyshttp

import (
	"crypto/ed25519"
	"encoding/base64"
	"reflect"
	"strings"
	"testing"

	"github.com/cthierer/canterbury/internal/domain/devauth"
)

func TestVerificationKeyToJWKBuildsEd25519Key(t *testing.T) {
	publicKey := ed25519.PublicKey([]byte("12345678901234567890123456789012"))

	got, err := verificationKeyToJWK(devauth.VerificationKey{
		ID:        "dev-key",
		Algorithm: devauth.SigningAlgorithmEdDSA,
		PublicKey: publicKey,
	})
	if err != nil {
		t.Fatalf("verificationKeyToJWK() error = %v", err)
	}

	want := key{
		Kty: "OKP",
		Use: "sig",
		Kid: "dev-key",
		Alg: "EdDSA",
		Crv: "Ed25519",
		X:   base64.RawURLEncoding.EncodeToString(publicKey),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("verificationKeyToJWK() = %#v, want %#v", got, want)
	}
}

func TestVerificationKeyToJWKRejectsUnsupportedKeys(t *testing.T) {
	_, err := verificationKeyToJWK(devauth.VerificationKey{PublicKey: "not a public key"})
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "unsupported key type") {
		t.Fatalf("verificationKeyToJWK() error = %q, want unsupported key type", err)
	}
}

func TestVerificationKeyToJWKRejectsInvalidEd25519Keys(t *testing.T) {
	publicKey := ed25519.PublicKey([]byte("too short"))

	_, err := verificationKeyToJWK(devauth.VerificationKey{PublicKey: publicKey})
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "invalid Ed25519 public key size") {
		t.Fatalf("verificationKeyToJWK() error = %q, want invalid Ed25519 size", err)
	}
}
