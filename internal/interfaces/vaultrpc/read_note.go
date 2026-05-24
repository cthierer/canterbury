package vaultrpc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"connectrpc.com/connect"
	vaultv1 "github.com/cthierer/canterbury/gen/go/canterbury/vault/v1"
	domainauth "github.com/cthierer/canterbury/internal/domain/auth"
	domainvault "github.com/cthierer/canterbury/internal/domain/vault"
)

// ReadNote handles read note requests.
func (h *VaultServiceHandler) ReadNote(
	ctx context.Context,
	req *connect.Request[vaultv1.ReadNoteRequest],
) (*connect.Response[vaultv1.ReadNoteResponse], error) {
	noteRef := req.Msg.Ref
	if noteRef == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("ref is a required parameter"))
	}

	notePath, err := domainvault.NewNotePath(noteRef.Path)
	if err != nil {
		slog.DebugContext(ctx, "could not convert path to NotePath", "err", err)
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("ref must contain a valid path"))
	}

	note, err := h.vault.ReadNote(ctx, notePath)
	if err != nil {
		connectErr := classifyReadNoteError(req.Msg, err)
		logConnectError(ctx, "encountered an error reading vault note", err, connectErr)
		return nil, connectErr
	}

	noteMsg, err := noteToProto(note)
	if err != nil {
		slog.ErrorContext(ctx, "encountered an error building note message", "err", err)
		return nil, connect.NewError(connect.CodeUnknown, errors.New("unexpected error building note message"))
	}

	resp := connect.NewResponse(&vaultv1.ReadNoteResponse{
		Note: noteMsg,
	})

	return resp, nil
}

func classifyReadNoteError(req *vaultv1.ReadNoteRequest, err error) error {
	if errors.Is(err, domainvault.ErrNoteNotFound) {
		return connect.NewError(connect.CodeNotFound, fmt.Errorf("requested note %q not found", req.Ref.Path))
	}

	if errors.Is(err, domainauth.ErrPermissionDenied) {
		return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("permission denied; check your authorization scopes"))
	}

	if errors.Is(err, domainvault.ErrInvalidNotePath) {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid note path %q", req.Ref.Path))
	}

	return classifySystemError(err)
}
