package vaultrpc

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"connectrpc.com/connect"
	vaultv1 "github.com/cthierer/canterbury/gen/go/canterbury/vault/v1"
	"github.com/cthierer/canterbury/internal/app/auditctx"
	"github.com/cthierer/canterbury/internal/app/auth"
	"github.com/cthierer/canterbury/internal/app/authctx"
	"github.com/cthierer/canterbury/internal/domain/audit"
	"github.com/cthierer/canterbury/internal/domain/vault"
)

func TestNewAuthContextInterceptor(t *testing.T) {
	t.Run("requires authenticator", func(t *testing.T) {
		_, err := NewAuthContextInterceptor(nil, &fakeAuditLogger{})
		if err == nil {
			t.Fatal("expected error")
		}

		if err.Error() != "authenticator is required" {
			t.Fatalf("NewAuthContextInterceptor() error = %v, want authenticator required", err)
		}
	})

	t.Run("requires audit logger", func(t *testing.T) {
		_, err := NewAuthContextInterceptor(&fakeAuthenticator{}, nil)
		if err == nil {
			t.Fatal("expected error")
		}

		if err.Error() != "audit logger is required" {
			t.Fatalf("NewAuthContextInterceptor() error = %v, want audit logger required", err)
		}
	})

	t.Run("creates interceptor", func(t *testing.T) {
		interceptor, err := NewAuthContextInterceptor(&fakeAuthenticator{}, &fakeAuditLogger{})
		if err != nil {
			t.Fatalf("NewAuthContextInterceptor() error = %v", err)
		}

		if interceptor == nil {
			t.Fatal("expected interceptor")
		}
	})
}

func TestAuthContextInterceptorWrapUnary(t *testing.T) {
	wantPrincipal := auth.Principal{
		Issuer:          "https://auth.example.test",
		Subject:         "user_123",
		SubjectHash:     "sha256:subject",
		Scopes:          []vault.Scope{"personal-agent"},
		MappingChecksum: "sha256:mapping",
	}
	authenticator := &fakeAuthenticator{principal: wantPrincipal}
	auditLog := &fakeAuditLogger{}
	interceptor := mustAuthInterceptor(t, authenticator, auditLog)
	req := connect.NewRequest(&vaultv1.ReadNoteRequest{})
	req.Header().Set("Authorization", "Bearer test-token")

	var gotPrincipal auth.Principal
	next := func(ctx context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		var ok bool
		gotPrincipal, ok = authctx.PrincipalFromContext(ctx)
		if !ok {
			t.Fatal("expected principal in context")
		}

		return connect.NewResponse(&vaultv1.ReadNoteResponse{}), nil
	}

	_, err := interceptor.WrapUnary(next)(context.Background(), req)
	if err != nil {
		t.Fatalf("WrapUnary() error = %v", err)
	}

	if authenticator.token != "test-token" {
		t.Fatalf("authenticator token = %q, want test-token", authenticator.token)
	}

	if gotPrincipal.Subject != wantPrincipal.Subject {
		t.Fatalf("principal subject = %q, want %q", gotPrincipal.Subject, wantPrincipal.Subject)
	}

	if gotPrincipal.MappingChecksum != wantPrincipal.MappingChecksum {
		t.Fatalf("mapping checksum = %q, want %q", gotPrincipal.MappingChecksum, wantPrincipal.MappingChecksum)
	}

	if len(auditLog.events) != 0 {
		t.Fatalf("got %d audit events, want none", len(auditLog.events))
	}
}

func TestAuthContextInterceptorRejectsInvalidAuthorization(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(http.Header)
		wantReason string
	}{
		{
			name:       "missing header",
			setup:      func(_ http.Header) {},
			wantReason: reasonMissingBearerToken,
		},
		{
			name: "multiple headers",
			setup: func(header http.Header) {
				header.Add("Authorization", "Bearer first")
				header.Add("Authorization", "Bearer second")
			},
			wantReason: reasonMalformedAuthorization,
		},
		{
			name: "wrong scheme",
			setup: func(header http.Header) {
				header.Set("Authorization", "Basic token")
			},
			wantReason: reasonMalformedAuthorization,
		},
		{
			name: "empty bearer",
			setup: func(header http.Header) {
				header.Set("Authorization", "Bearer ")
			},
			wantReason: reasonMalformedAuthorization,
		},
		{
			name: "token contains whitespace",
			setup: func(header http.Header) {
				header.Set("Authorization", "Bearer token with spaces")
			},
			wantReason: reasonMalformedAuthorization,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			authenticator := &fakeAuthenticator{}
			auditLog := &fakeAuditLogger{}
			interceptor := mustAuthInterceptor(t, authenticator, auditLog)
			req := connect.NewRequest(&vaultv1.ReadNoteRequest{})
			test.setup(req.Header())

			nextCalled := false
			next := func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
				nextCalled = true
				return connect.NewResponse(&vaultv1.ReadNoteResponse{}), nil
			}

			_, err := interceptor.WrapUnary(next)(context.Background(), req)
			assertConnectCode(t, err, connect.CodeUnauthenticated)

			if nextCalled {
				t.Fatal("next should not be called")
			}

			if authenticator.token != "" {
				t.Fatalf("authenticator token = %q, want empty", authenticator.token)
			}

			assertAuthFailedEvent(t, auditLog.events, test.wantReason)
		})
	}
}

func TestAuthContextInterceptorMapsAuthenticateErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		want       connect.Code
		wantAudit  bool
		wantReason string
	}{
		{
			name:       "auth failure",
			err:        errors.New("bad token"),
			want:       connect.CodeUnauthenticated,
			wantAudit:  true,
			wantReason: reasonPrincipalResolutionError,
		},
		{
			name:       "principal resolution failure",
			err:        auth.ErrPrincipalResolutionFailed,
			want:       connect.CodeUnauthenticated,
			wantAudit:  true,
			wantReason: reasonPrincipalResolutionError,
		},
		{
			name:       "expired token",
			err:        auth.ErrExpiredToken,
			want:       connect.CodeUnauthenticated,
			wantAudit:  true,
			wantReason: reasonExpiredToken,
		},
		{
			name: "canceled",
			err:  context.Canceled,
			want: connect.CodeCanceled,
		},
		{
			name: "deadline exceeded",
			err:  context.DeadlineExceeded,
			want: connect.CodeDeadlineExceeded,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			auditLog := &fakeAuditLogger{}
			interceptor := mustAuthInterceptor(t, &fakeAuthenticator{err: test.err}, auditLog)
			req := connect.NewRequest(&vaultv1.ReadNoteRequest{})
			req.Header().Set("Authorization", "Bearer test-token")

			nextCalled := false
			next := func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
				nextCalled = true
				return connect.NewResponse(&vaultv1.ReadNoteResponse{}), nil
			}

			_, err := interceptor.WrapUnary(next)(context.Background(), req)
			assertConnectCode(t, err, test.want)

			if nextCalled {
				t.Fatal("next should not be called")
			}

			if test.wantAudit {
				assertAuthFailedEvent(t, auditLog.events, test.wantReason)
			} else if len(auditLog.events) != 0 {
				t.Fatalf("got %d audit events, want none", len(auditLog.events))
			}
		})
	}
}

func TestAuthContextInterceptorReturnsInternalWhenAuthAuditFails(t *testing.T) {
	auditErr := errors.New("audit unavailable")
	interceptor := mustAuthInterceptor(t, &fakeAuthenticator{}, &fakeAuditLogger{err: auditErr})
	req := connect.NewRequest(&vaultv1.ReadNoteRequest{})

	nextCalled := false
	next := func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
		nextCalled = true
		return connect.NewResponse(&vaultv1.ReadNoteResponse{}), nil
	}

	_, err := interceptor.WrapUnary(next)(context.Background(), req)
	assertConnectCode(t, err, connect.CodeInternal)

	if nextCalled {
		t.Fatal("next should not be called")
	}
}

func TestAuthContextInterceptorUsesAuditMetadata(t *testing.T) {
	auditLog := &fakeAuditLogger{}
	interceptor := mustAuthInterceptor(t, &fakeAuthenticator{}, auditLog)
	req := connect.NewRequest(&vaultv1.ReadNoteRequest{})
	requestID, err := audit.NewRequestID("req_test")
	if err != nil {
		t.Fatalf("make request ID: %v", err)
	}

	ctx := auditctx.WithMetadata(context.Background(), auditctx.Metadata{
		RequestID: requestID,
		Client: audit.Client{
			Interface: audit.ClientInterfaceConnectRPC,
			UserAgent: "test-agent",
		},
	})

	next := func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
		t.Fatal("next should not be called")
		return nil, nil
	}

	_, err = interceptor.WrapUnary(next)(ctx, req)
	assertConnectCode(t, err, connect.CodeUnauthenticated)
	assertAuthFailedEvent(t, auditLog.events, reasonMissingBearerToken)

	if auditLog.events[0].RequestID != requestID {
		t.Fatalf("RequestID = %q, want %q", auditLog.events[0].RequestID, requestID)
	}

	if auditLog.events[0].Client.UserAgent != "test-agent" {
		t.Fatalf("UserAgent = %q, want test-agent", auditLog.events[0].Client.UserAgent)
	}
}

func TestBearerTokenFromHeader(t *testing.T) {
	t.Run("accepts case-insensitive bearer scheme", func(t *testing.T) {
		header := http.Header{}
		header.Set("Authorization", "bearer test-token")

		got, err := bearerTokenFromHeader(header)
		if err != nil {
			t.Fatalf("bearerTokenFromHeader() error = %v", err)
		}

		if got != "test-token" {
			t.Fatalf("token = %q, want test-token", got)
		}
	})

	t.Run("trims surrounding whitespace", func(t *testing.T) {
		header := http.Header{}
		header.Set("Authorization", " Bearer test-token ")

		got, err := bearerTokenFromHeader(header)
		if err != nil {
			t.Fatalf("bearerTokenFromHeader() error = %v", err)
		}

		if got != "test-token" {
			t.Fatalf("token = %q, want test-token", got)
		}
	})
}

func TestAuthFailureReason(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "missing bearer", err: errMissingBearerToken, want: reasonMissingBearerToken},
		{name: "malformed authorization", err: errMalformedAuthorization, want: reasonMalformedAuthorization},
		{name: "multiple headers", err: errMultipleHeaders, want: reasonMalformedAuthorization},
		{name: "malformed jwt", err: auth.ErrMalformedToken, want: reasonMalformedJWT},
		{name: "invalid signature", err: auth.ErrInvalidTokenSignature, want: reasonInvalidSignature},
		{name: "expired token", err: auth.ErrExpiredToken, want: reasonExpiredToken},
		{name: "wrong issuer", err: auth.ErrWrongIssuer, want: reasonWrongIssuer},
		{name: "wrong audience", err: auth.ErrWrongAudience, want: reasonWrongAudience},
		{name: "missing subject", err: auth.ErrMissingSubject, want: reasonMissingSubject},
		{name: "unknown", err: errors.New("unknown auth failure"), want: reasonPrincipalResolutionError},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := authFailureReason(test.err); got != test.want {
				t.Fatalf("authFailureReason() = %q, want %q", got, test.want)
			}
		})
	}
}

func mustAuthInterceptor(t *testing.T, authenticator Authenticator, auditLog AuditLogger) *authContextInterceptor {
	t.Helper()

	interceptor, err := NewAuthContextInterceptor(authenticator, auditLog)
	if err != nil {
		t.Fatalf("NewAuthContextInterceptor() error = %v", err)
	}

	authInterceptor, ok := interceptor.(*authContextInterceptor)
	if !ok {
		t.Fatalf("interceptor = %T, want *authContextInterceptor", interceptor)
	}

	return authInterceptor
}

func assertAuthFailedEvent(t *testing.T, events []audit.Event, wantReason string) {
	t.Helper()

	if len(events) != 1 {
		t.Fatalf("got %d audit events, want 1", len(events))
	}

	event := events[0]
	if event.Type() != audit.EventTypeAuthFailed {
		t.Fatalf("event type = %q, want %q", event.Type(), audit.EventTypeAuthFailed)
	}

	if event.Policy.Decision != audit.PolicyDecisionDeny {
		t.Fatalf("policy decision = %q, want deny", event.Policy.Decision)
	}

	if event.Policy.MappingChecksum != "sha256:mapping" {
		t.Fatalf("mapping checksum = %q, want sha256:mapping", event.Policy.MappingChecksum)
	}

	if event.Outcome.Status != audit.OutcomeStatusFailed {
		t.Fatalf("outcome status = %q, want failed", event.Outcome.Status)
	}

	if event.Outcome.Code != audit.OutcomeCodeUnauthenticated {
		t.Fatalf("outcome code = %q, want unauthenticated", event.Outcome.Code)
	}

	details, ok := event.Details.(authFailedDetails)
	if !ok {
		t.Fatalf("details = %T, want authFailedDetails", event.Details)
	}

	if details.Reason != wantReason {
		t.Fatalf("reason = %q, want %q", details.Reason, wantReason)
	}
}

type fakeAuthenticator struct {
	token     string
	principal auth.Principal
	err       error
}

func (authenticator *fakeAuthenticator) Authenticate(ctx context.Context, token string) (auth.Principal, error) {
	if err := ctx.Err(); err != nil {
		return auth.Principal{}, err
	}

	authenticator.token = token

	if authenticator.err != nil {
		return auth.Principal{}, authenticator.err
	}

	return authenticator.principal, nil
}

func (authenticator *fakeAuthenticator) MappingChecksum() string {
	return "sha256:mapping"
}

type fakeAuditLogger struct {
	events []audit.Event
	err    error
}

func (logger *fakeAuditLogger) RecordEvent(_ context.Context, event audit.Event) error {
	if logger.err != nil {
		return logger.err
	}

	logger.events = append(logger.events, event)
	return nil
}
