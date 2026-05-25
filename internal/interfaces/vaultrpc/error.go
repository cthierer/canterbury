package vaultrpc

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	domainauth "github.com/cthierer/canterbury/internal/domain/auth"
	domainvault "github.com/cthierer/canterbury/internal/domain/vault"
)

func classifySystemError(err error) error {
	if errors.Is(err, domainvault.ErrVaultUnavailable) {
		return connect.NewError(connect.CodeUnavailable, fmt.Errorf("vault cannot be accessed; try again later"))
	}

	if errors.Is(err, domainauth.ErrMissingPrincipal) {
		return connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("missing authentication"))
	}

	if errors.Is(err, context.Canceled) {
		return connect.NewError(connect.CodeCanceled, fmt.Errorf("request canceled"))
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return connect.NewError(connect.CodeDeadlineExceeded, fmt.Errorf("deadline exceeded"))
	}

	return connect.NewError(connect.CodeUnknown, fmt.Errorf("an unknown error occurred"))
}
