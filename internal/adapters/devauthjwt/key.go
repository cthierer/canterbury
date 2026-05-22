package devauthjwt

import (
	"crypto/ed25519"

	"github.com/cthierer/canterbury/internal/domain/devauth"
)

// VerificationKey returns the public key corresponding to minted JWTs.
func (minter *Minter) VerificationKey() devauth.VerificationKey {
	publicKey := append(ed25519.PublicKey(nil), minter.publicKey...)

	return devauth.VerificationKey{
		ID:        minter.keyID,
		Algorithm: devauth.SigningAlgorithmEdDSA,
		PublicKey: publicKey,
	}
}
