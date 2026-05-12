package devauthjwt

import "github.com/cthierer/canterbury/internal/domain/devauth"

// VerificationKey returns the public key corresponding to minted JWTs.
func (minter *Minter) VerificationKey() devauth.VerificationKey {
	return devauth.VerificationKey{
		ID:        minter.keyID,
		Algorithm: devauth.SigningAlgorithmEdDSA,
		PublicKey: minter.publicKey,
	}
}
