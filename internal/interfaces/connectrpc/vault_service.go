package connectrpc

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	vaultv1 "github.com/cthierer/canterbury/gen/go/canterbury/vault/v1"
	"github.com/cthierer/canterbury/gen/go/canterbury/vault/v1/vaultv1connect"
)

var _ vaultv1connect.VaultServiceHandler = (*VaultServiceHandler)(nil)

// VaultServiceHandler is the Connect transport adapter for vault RPCs.
type VaultServiceHandler struct{}

// NewVaultServiceHandler creates a Connect vault service handler.
func NewVaultServiceHandler() *VaultServiceHandler {
	return &VaultServiceHandler{}
}

// ReadNote handles read note requests.
func (h *VaultServiceHandler) ReadNote(
	context.Context,
	*connect.Request[vaultv1.ReadNoteRequest],
) (*connect.Response[vaultv1.ReadNoteResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("read note is not implemented"))
}

// SearchNotes handles search notes requests.
func (h *VaultServiceHandler) SearchNotes(
	context.Context,
	*connect.Request[vaultv1.SearchNotesRequest],
) (*connect.Response[vaultv1.SearchNotesResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("search notes is not implemented"))
}
