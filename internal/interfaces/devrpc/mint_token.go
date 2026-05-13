package devrpc

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	devv1 "github.com/cthierer/canterbury/gen/go/canterbury/dev/v1"
)

// MintToken handles development token minting requests.
func (service *DevAuthServiceHandler) MintToken(
	_ context.Context,
	_ *connect.Request[devv1.MintTokenRequest],
) (*connect.Response[devv1.MintTokenResponse], error) {
	// TODO implement
	return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("coming soon"))
}
