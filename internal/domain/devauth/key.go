package devauth

import "crypto"

// SigningAlgorithm names the token signing algorithm used by a verification key.
type SigningAlgorithm string

func (s SigningAlgorithm) String() string {
	return string(s)
}

// SigningAlgorithmEdDSA identifies EdDSA-signed development tokens.
const SigningAlgorithmEdDSA SigningAlgorithm = "EdDSA"

// VerificationKey describes public key material used to verify minted tokens.
type VerificationKey struct {
	ID        string
	Algorithm SigningAlgorithm
	PublicKey crypto.PublicKey
}
