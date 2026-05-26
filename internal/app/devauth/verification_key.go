package devauth

import domain "github.com/cthierer/canterbury/internal/domain/devauth"

// VerificationKey returns the public key used to verify minted tokens.
func (service *Service) VerificationKey() domain.VerificationKey {
	return service.minter.VerificationKey()
}
