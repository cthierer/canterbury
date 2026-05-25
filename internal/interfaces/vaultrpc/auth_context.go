package vaultrpc

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/cthierer/canterbury/internal/app/auditctx"
	appauth "github.com/cthierer/canterbury/internal/app/auth"
	"github.com/cthierer/canterbury/internal/app/authctx"
	"github.com/cthierer/canterbury/internal/app/clock"
	"github.com/cthierer/canterbury/internal/domain/audit"
	"github.com/cthierer/canterbury/internal/domain/vault"
)

type authContextInterceptor struct {
	authenticator Authenticator
	auditLog      AuditLogger
	clock         clock.Clock
}

var _ connect.Interceptor = (*authContextInterceptor)(nil)

// Authenticator resolves bearer tokens into application principals.
type Authenticator interface {
	Authenticate(ctx context.Context, token string) (appauth.Principal, error)
	MappingChecksum() string
}

// AuditLogger records audit events produced by the auth interceptor.
type AuditLogger interface {
	RecordEvent(ctx context.Context, event audit.Event) error
}

// NewAuthContextInterceptor creates a Connect interceptor that authenticates
// bearer tokens and attaches the resolved principal to the request context.
func NewAuthContextInterceptor(authenticator Authenticator, auditLog AuditLogger) (connect.Interceptor, error) {
	if authenticator == nil {
		return nil, fmt.Errorf("authenticator is required")
	}

	if auditLog == nil {
		return nil, fmt.Errorf("audit logger is required")
	}

	return &authContextInterceptor{
		authenticator: authenticator,
		auditLog:      auditLog,
		clock:         clock.SystemClock{},
	}, nil
}

func (interceptor *authContextInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		startedAt := interceptor.clock.Now()

		token, err := bearerTokenFromHeader(req.Header())
		if err != nil {
			if auditErr := interceptor.recordAuthFailed(ctx, err, startedAt); auditErr != nil {
				return nil, handleAuthAuditError(auditErr)
			}

			return nil, handleAuthInterceptorError(err)
		}

		principal, err := interceptor.authenticator.Authenticate(ctx, token)
		if err != nil {
			if shouldRecordAuthFailed(err) {
				if auditErr := interceptor.recordAuthFailed(ctx, err, startedAt); auditErr != nil {
					return nil, handleAuthAuditError(auditErr)
				}
			}

			return nil, handleAuthInterceptorError(err)
		}

		ctx = authctx.WithPrincipal(ctx, principal)
		return next(ctx, req)
	}
}

func (interceptor *authContextInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (interceptor *authContextInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}

var (
	errMultipleHeaders        = fmt.Errorf("more than one authorization header")
	errMissingBearerToken     = fmt.Errorf("missing bearer token")
	errMalformedAuthorization = fmt.Errorf("malformed authorization header value")
)

const (
	reasonMissingBearerToken       string = "missing_bearer_token"
	reasonMalformedAuthorization   string = "malformed_authorization"
	reasonMalformedJWT             string = "malformed_jwt"
	reasonInvalidSignature         string = "invalid_signature"
	reasonExpiredToken             string = "expired_token"
	reasonWrongIssuer              string = "wrong_issuer"
	reasonWrongAudience            string = "wrong_audience"
	reasonMissingSubject           string = "missing_subject"
	reasonPrincipalResolutionError string = "principal_resolution_failed"
)

type authFailedDetails struct {
	Reason string `json:"reason"`
}

func (authFailedDetails) EventType() audit.EventType {
	return audit.EventTypeAuthFailed
}

func bearerTokenFromHeader(header http.Header) (string, error) {
	values := header.Values("authorization")
	if len(values) > 1 {
		return "", errMultipleHeaders
	}

	if len(values) == 0 {
		return "", errMissingBearerToken
	}

	value := strings.TrimSpace(values[0])
	if value == "" {
		return "", errMissingBearerToken
	}

	scheme, token, ok := strings.Cut(value, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", errMalformedAuthorization
	}

	token = strings.TrimSpace(token)
	if token == "" || strings.ContainsAny(token, " \t\r\n") {
		return "", errMalformedAuthorization
	}

	return token, nil
}

func handleAuthInterceptorError(err error) error {
	switch {
	case errors.Is(err, errMissingBearerToken), errors.Is(err, errMalformedAuthorization), errors.Is(err, errMultipleHeaders):
		return connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid authorization"))
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return classifySystemError(err)
	default:
		return connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication failed"))
	}
}

func handleAuthAuditError(_ error) error {
	return connect.NewError(connect.CodeInternal, fmt.Errorf("authentication failed"))
}

func shouldRecordAuthFailed(err error) bool {
	return !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
}

func (interceptor *authContextInterceptor) recordAuthFailed(ctx context.Context, err error, startedAt time.Time) error {
	metadata, ok := auditctx.MetadataFromContext(ctx)
	client := metadata.Client
	if !ok {
		client.Interface = audit.ClientInterfaceConnectRPC
	}

	event := audit.Event{
		OccurredAt: startedAt.UTC(),
		RequestID:  metadata.RequestID,
		TraceID:    metadata.TraceID,
		Client:     client,
		Policy: audit.Policy{
			MappingChecksum: interceptor.authenticator.MappingChecksum(),
			MatchedScopes:   []vault.Scope{},
			Decision:        audit.PolicyDecisionDeny,
		},
		Outcome: audit.Outcome{
			Status:   audit.OutcomeStatusFailed,
			Code:     audit.OutcomeCodeUnauthenticated,
			Duration: interceptor.clock.Now().Sub(startedAt),
		},
		Details: authFailedDetails{
			Reason: authFailureReason(err),
		},
	}

	if recordErr := interceptor.auditLog.RecordEvent(ctx, event); recordErr != nil {
		return fmt.Errorf("record auth failed event: %w", recordErr)
	}

	return nil
}

func authFailureReason(err error) string {
	switch {
	case errors.Is(err, errMissingBearerToken):
		return reasonMissingBearerToken
	case errors.Is(err, errMalformedAuthorization), errors.Is(err, errMultipleHeaders):
		return reasonMalformedAuthorization
	case errors.Is(err, appauth.ErrMalformedToken):
		return reasonMalformedJWT
	case errors.Is(err, appauth.ErrInvalidTokenSignature):
		return reasonInvalidSignature
	case errors.Is(err, appauth.ErrExpiredToken):
		return reasonExpiredToken
	case errors.Is(err, appauth.ErrWrongIssuer):
		return reasonWrongIssuer
	case errors.Is(err, appauth.ErrWrongAudience):
		return reasonWrongAudience
	case errors.Is(err, appauth.ErrMissingSubject):
		return reasonMissingSubject
	default:
		return reasonPrincipalResolutionError
	}
}
