package keyshttp

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"

	"github.com/cthierer/canterbury/internal/domain/devauth"
)

func (service *KeyStoreServiceHandler) getKeySet(ctx context.Context) (keySet, error) {
	if err := ctx.Err(); err != nil {
		return keySet{}, err
	}

	verificationKey := service.keyStore.VerificationKey()
	jwk, err := verificationKeyToJWK(verificationKey)
	if err != nil {
		return keySet{}, fmt.Errorf("convert verification key to JWK: %w", err)
	}

	jwks := keySet{Keys: []key{jwk}}
	return jwks, nil
}

func verificationKeyToJWK(verificationKey devauth.VerificationKey) (key, error) {
	jwk := key{
		Kid: verificationKey.ID,
		Use: "sig",
	}

	switch k := verificationKey.PublicKey.(type) {
	case *ed25519.PublicKey:
		if k == nil {
			return key{}, fmt.Errorf("public key is nil")
		}

		var err error
		jwk, err = setEd25519Key(jwk, verificationKey.Algorithm, *k)
		if err != nil {
			return key{}, fmt.Errorf("set ed25519 key: %w", err)
		}
	case ed25519.PublicKey:
		var err error
		jwk, err = setEd25519Key(jwk, verificationKey.Algorithm, k)
		if err != nil {
			return key{}, fmt.Errorf("set ed25519 key: %w", err)
		}
	default:
		return key{}, fmt.Errorf("unsupported key type: %T", verificationKey.PublicKey)
	}

	return jwk, nil
}

func setEd25519Key(jwk key, algorithm devauth.SigningAlgorithm, publicKey ed25519.PublicKey) (key, error) {
	if algorithm != devauth.SigningAlgorithmEdDSA {
		return key{}, fmt.Errorf("unsupported signing algorithm for Ed25519 public key: %q", algorithm)
	}

	if len(publicKey) != ed25519.PublicKeySize {
		return key{}, fmt.Errorf("invalid Ed25519 public key size: expected %d bytes, got %d bytes", ed25519.PublicKeySize, len(publicKey))
	}

	jwk.Kty = "OKP"
	jwk.Alg = algorithm.String()
	jwk.Crv = "Ed25519"
	jwk.X = base64.RawURLEncoding.EncodeToString([]byte(publicKey))

	return jwk, nil
}
