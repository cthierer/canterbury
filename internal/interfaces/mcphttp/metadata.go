package mcphttp

import (
	"context"
	"errors"

	"connectrpc.com/connect"
)

const (
	headerRequestID   = "X-Request-ID"
	headerTraceParent = "traceparent"
)

// NewForwardMetadataInterceptor forwards request identity and correlation
// metadata from the MCP HTTP request to each vault RPC.
func NewForwardMetadataInterceptor(userAgent string) connect.Interceptor {
	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
			metadata, ok := ctx.Value(requestMetadataKey{}).(requestMetadata)
			if !ok || metadata.bearerToken == "" {
				return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing forwarded authorization"))
			}

			request.Header().Set(headerAuthorization, "Bearer "+metadata.bearerToken)
			request.Header().Set("User-Agent", userAgent)
			if metadata.requestID != "" {
				request.Header().Set(headerRequestID, metadata.requestID)
			}
			if metadata.traceParent != "" {
				request.Header().Set(headerTraceParent, metadata.traceParent)
			}

			return next(ctx, request)
		}
	})
}
