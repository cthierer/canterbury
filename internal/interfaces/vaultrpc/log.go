package vaultrpc

import (
	"context"
	"log/slog"

	"connectrpc.com/connect"
)

func logConnectError(ctx context.Context, message string, err error, connectErr error) {
	switch connect.CodeOf(connectErr) {
	case connect.CodeInvalidArgument, connect.CodeNotFound, connect.CodePermissionDenied, connect.CodeCanceled, connect.CodeDeadlineExceeded:
		slog.DebugContext(ctx, message, "err", err)
	default:
		slog.ErrorContext(ctx, message, "err", err)
	}
}
