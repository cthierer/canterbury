package vaultrpc

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"

	"connectrpc.com/connect"
	"github.com/cthierer/canterbury/internal/app/auditctx"
	"github.com/cthierer/canterbury/internal/app/clock"
	"github.com/cthierer/canterbury/internal/domain/audit"
)

type auditContextInterceptor struct {
	clock   clock.Clock
	hmacKey []byte
}

var _ connect.Interceptor = (*auditContextInterceptor)(nil)

// NewAuditContextInterceptor creates a Connect interceptor that attaches
// request-scoped audit metadata to unary vault RPCs.
func NewAuditContextInterceptor(hmacKey []byte) (connect.Interceptor, error) {
	if len(hmacKey) < 32 {
		return nil, fmt.Errorf("audit HMAC key must be at least 32 bytes")
	}

	return &auditContextInterceptor{
		clock:   clock.SystemClock{},
		hmacKey: append([]byte(nil), hmacKey...),
	}, nil
}

func (interceptor *auditContextInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		reqID, err := interceptor.requestID(req)
		if err != nil {
			return nil, handleAuditInterceptorError(err)
		}

		traceID, _ := interceptor.traceID(req)
		// TODO log debug about invalid trace ID, but don't quit

		client, err := interceptor.client(req)
		if err != nil {
			return nil, withRequestID(handleAuditInterceptorError(err), reqID)
		}

		metadata := auditctx.Metadata{
			RequestID: reqID,
			TraceID:   traceID,
			Client:    client,
		}

		ctx = auditctx.WithMetadata(ctx, metadata)

		res, err := next(ctx, req)
		if err != nil {
			return nil, withRequestID(err, reqID)
		}

		interceptor.setRequestID(res, reqID)

		return res, nil
	}
}

func (interceptor *auditContextInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (interceptor *auditContextInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}

func (interceptor *auditContextInterceptor) client(req connect.AnyRequest) (audit.Client, error) {
	userAgent := userAgentFromHeader(req.Header())

	remoteAddressHash, err := hashRemoteAddress(req.Peer().Addr, interceptor.hmacKey)
	if err != nil {
		return audit.Client{}, fmt.Errorf("hash remote address: %w", err)
	}

	return audit.Client{
		Interface:         audit.ClientInterfaceConnectRPC,
		UserAgent:         userAgent,
		RemoteAddressHash: remoteAddressHash,
	}, nil
}

func handleAuditInterceptorError(err error) error {
	switch {
	case errors.Is(err, audit.ErrInvalidRequestID):
		return connect.NewError(connect.CodeInvalidArgument, errors.New("request ID is not valid"))
	default:
		return classifySystemError(err)
	}
}

func withRequestID(err error, reqID audit.RequestID) error {
	var connectErr *connect.Error
	if errors.As(err, &connectErr) {
		connectErr.Meta().Set(headerRequestID, reqID.String())
		return connectErr
	}

	connectErr = connect.NewError(connect.CodeUnknown, err)
	connectErr.Meta().Set(headerRequestID, reqID.String())
	return connectErr
}

func userAgentFromHeader(header http.Header) string {
	headerVal := header.Get("user-agent")
	return strings.TrimSpace(headerVal)
}

func hashRemoteAddress(remoteAddress string, key []byte) (string, error) {
	host, _, err := net.SplitHostPort(remoteAddress)
	if err != nil {
		host = remoteAddress
	}

	addr, err := netip.ParseAddr(host)
	if err != nil {
		return "", fmt.Errorf("parse remote address: %w", err)
	}

	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(addr.String()))

	return "hmac-sha256:" + hex.EncodeToString(mac.Sum(nil)), nil
}
