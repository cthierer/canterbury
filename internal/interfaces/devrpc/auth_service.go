// Package devrpc adapts development auth application services to Connect RPC.
package devrpc

import (
	"context"
	"fmt"

	"github.com/cthierer/canterbury/gen/go/canterbury/dev/v1/devv1connect"
	"github.com/cthierer/canterbury/internal/domain/devauth"
)

var _ devv1connect.DevAuthServiceHandler = (*DevAuthServiceHandler)(nil)

// AuthApplication defines the development auth application behavior used by RPC handlers.
type AuthApplication interface {
	MintToken(ctx context.Context, claims devauth.Claims, options devauth.MintOptions) (devauth.Token, error)
	VerificationKey() devauth.VerificationKey
}

// DevAuthServiceHandler implements the generated Connect development auth service.
type DevAuthServiceHandler struct {
	auth AuthApplication
}

// NewDevAuthServiceHandler creates a Connect development auth service handler.
func NewDevAuthServiceHandler(auth AuthApplication) (*DevAuthServiceHandler, error) {
	if auth == nil {
		return nil, fmt.Errorf("auth application service is required")
	}

	return &DevAuthServiceHandler{auth: auth}, nil
}
