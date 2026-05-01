package connectrpc

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	vaultv1 "github.com/cthierer/canterbury/gen/go/canterbury/vault/v1"
	"github.com/cthierer/canterbury/gen/go/canterbury/vault/v1/vaultv1connect"
	domainvault "github.com/cthierer/canterbury/internal/domain/vault"
)

var _ vaultv1connect.VaultServiceHandler = (*VaultServiceHandler)(nil)

type VaultApplication interface {
	ReadNote(ctx context.Context, path domainvault.NotePath) (domainvault.Note, error)
	SearchNotes(ctx context.Context, query domainvault.SearchNotesQuery) (domainvault.SearchNotesPage, error)
}

// VaultServiceHandler is the Connect transport adapter for vault RPCs.
type VaultServiceHandler struct {
	vault VaultApplication
}

// NewVaultServiceHandler creates a Connect vault service handler.
func NewVaultServiceHandler(vault VaultApplication) (*VaultServiceHandler, error) {
	if vault == nil {
		return nil, fmt.Errorf("vault application service is required")
	}

	return &VaultServiceHandler{vault: vault}, nil
}

// SearchNotes handles search notes requests.
func (h *VaultServiceHandler) SearchNotes(
	context.Context,
	*connect.Request[vaultv1.SearchNotesRequest],
) (*connect.Response[vaultv1.SearchNotesResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("search notes is not implemented"))
}
