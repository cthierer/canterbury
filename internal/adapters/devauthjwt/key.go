package devauthjwt

import "github.com/cthierer/canterbury/internal/domain/devauth"

// VerificationKey returns the public key corresponding to minted JWTs.
func (minter *Minter) VerificationKey() devauth.VerificationKey {
	publicKey := append([]byte(nil), minter.publicKey...)

	return devauth.VerificationKey{
		ID:        minter.keyID,
		Algorithm: devauth.SigningAlgorithmEdDSA,
		PublicKey: publicKey,
	}
}
