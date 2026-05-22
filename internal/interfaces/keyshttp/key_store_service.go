package keyshttp

import (
	"fmt"

	"github.com/cthierer/canterbury/internal/domain/devauth"
)

// KeyStoreApplication defines the development auth key behavior used by HTTP handlers.
type KeyStoreApplication interface {
	VerificationKey() devauth.VerificationKey
}

// KeyStoreServiceHandler serves public development auth verification keys.
type KeyStoreServiceHandler struct {
	keyStore KeyStoreApplication
}

// NewKeyStoreServiceHandler creates an HTTP handler for a key store application.
func NewKeyStoreServiceHandler(keyStore KeyStoreApplication) (*KeyStoreServiceHandler, error) {
	if keyStore == nil {
		return nil, fmt.Errorf("key store service is required")
	}

	return &KeyStoreServiceHandler{keyStore: keyStore}, nil
}
