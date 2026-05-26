package devrpc

import (
	"context"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	devv1 "github.com/cthierer/canterbury/gen/go/canterbury/dev/v1"
	"github.com/cthierer/canterbury/internal/domain/devauth"
)

func TestNewDevAuthServiceHandler(t *testing.T) {
	t.Run("rejects missing application service", func(t *testing.T) {
		_, err := NewDevAuthServiceHandler(nil)
		if err == nil {
			t.Fatal("expected error")
		}

		if !strings.Contains(err.Error(), "auth application service is required") {
			t.Fatalf("NewDevAuthServiceHandler() error = %q, want required service message", err)
		}
	})

	t.Run("stores application service", func(t *testing.T) {
		auth := &fakeAuthApplication{}

		got, err := NewDevAuthServiceHandler(auth)
		if err != nil {
			t.Fatalf("NewDevAuthServiceHandler() error = %v", err)
		}

		if got.auth != auth {
			t.Fatalf("handler auth = %#v, want fake application", got.auth)
		}
	})
}

func TestMintTokenMapsRequestToApplication(t *testing.T) {
	expiresAt := time.Date(2026, 5, 19, 21, 0, 0, 0, time.UTC)
	auth := &fakeAuthApplication{
		token: devauth.Token{
			JWT:       "token",
			Type:      devauth.TokenTypeBearer,
			ExpiresAt: expiresAt,
		},
	}
	handler, err := NewDevAuthServiceHandler(auth)
	if err != nil {
		t.Fatalf("NewDevAuthServiceHandler() error = %v", err)
	}

	resp, err := handler.MintToken(t.Context(), connect.NewRequest(&devv1.MintTokenRequest{
		Claims: &devv1.TokenClaims{
			Subject:   "user_123",
			Audiences: []string{"canterbury"},
		},
		Options: &devv1.MintTokenOptions{
			TtlSeconds: 300,
		},
	}))
	if err != nil {
		t.Fatalf("MintToken() error = %v", err)
	}

	wantClaims := devauth.Claims{
		Subject:   "user_123",
		Audiences: []string{"canterbury"},
	}
	if !reflect.DeepEqual(auth.claims, wantClaims) {
		t.Fatalf("application claims = %#v, want %#v", auth.claims, wantClaims)
	}

	if auth.options.TTL != 5*time.Minute {
		t.Fatalf("application TTL = %v, want %v", auth.options.TTL, 5*time.Minute)
	}

	if resp.Msg.Token.Jwt != "token" {
		t.Fatalf("response JWT = %q, want %q", resp.Msg.Token.Jwt, "token")
	}
	if resp.Msg.Token.TokenType != "Bearer" {
		t.Fatalf("response token type = %q, want %q", resp.Msg.Token.TokenType, "Bearer")
	}
	if !resp.Msg.Token.ExpiresAt.AsTime().Equal(expiresAt) {
		t.Fatalf("response expiry = %v, want %v", resp.Msg.Token.ExpiresAt.AsTime(), expiresAt)
	}
}

func TestMintTokenUsesDefaultOptionsWhenOptionsAreOmitted(t *testing.T) {
	auth := &fakeAuthApplication{}
	handler, err := NewDevAuthServiceHandler(auth)
	if err != nil {
		t.Fatalf("NewDevAuthServiceHandler() error = %v", err)
	}

	_, err = handler.MintToken(t.Context(), connect.NewRequest(&devv1.MintTokenRequest{
		Claims: &devv1.TokenClaims{
			Subject:   "user_123",
			Audiences: []string{"canterbury"},
		},
	}))
	if err != nil {
		t.Fatalf("MintToken() error = %v", err)
	}

	if auth.options.TTL != 0 {
		t.Fatalf("application TTL = %v, want default sentinel 0", auth.options.TTL)
	}
}

func TestMintTokenClassifiesApplicationValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want connect.Code
	}{
		{
			name: "missing subject",
			err:  devauth.ErrMissingSubject,
			want: connect.CodeInvalidArgument,
		},
		{
			name: "missing audience",
			err:  devauth.ErrMissingAudience,
			want: connect.CodeInvalidArgument,
		},
		{
			name: "negative ttl",
			err:  devauth.ErrNegativeTTL,
			want: connect.CodeInvalidArgument,
		},
		{
			name: "large ttl",
			err:  devauth.ErrLargeTTL,
			want: connect.CodeInvalidArgument,
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
		{
			name: "unknown",
			err:  errors.New("signing backend failed"),
			want: connect.CodeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := &fakeAuthApplication{
				mintErr: tt.err,
			}
			handler, err := NewDevAuthServiceHandler(auth)
			if err != nil {
				t.Fatalf("NewDevAuthServiceHandler() error = %v", err)
			}

			_, err = handler.MintToken(t.Context(), connect.NewRequest(validMintTokenRequest()))
			assertConnectCode(t, err, tt.want)
		})
	}
}

func TestMintTokenRejectsTTLBeforeDurationOverflow(t *testing.T) {
	maxDurationSeconds := math.MaxInt64 / int64(time.Second)

	tests := []struct {
		name       string
		ttlSeconds int64
		wantErr    bool
	}{
		{
			name:       "max representable ttl",
			ttlSeconds: maxDurationSeconds,
		},
		{
			name:       "negative ttl",
			ttlSeconds: -1,
			wantErr:    true,
		},
		{
			name:       "ttl exceeds duration range",
			ttlSeconds: maxDurationSeconds + 1,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := &fakeAuthApplication{}
			handler, err := NewDevAuthServiceHandler(auth)
			if err != nil {
				t.Fatalf("NewDevAuthServiceHandler() error = %v", err)
			}

			_, err = handler.MintToken(t.Context(), connect.NewRequest(&devv1.MintTokenRequest{
				Claims: &devv1.TokenClaims{
					Subject:   "user_123",
					Audiences: []string{"canterbury"},
				},
				Options: &devv1.MintTokenOptions{
					TtlSeconds: tt.ttlSeconds,
				},
			}))
			if tt.wantErr {
				assertConnectCode(t, err, connect.CodeInvalidArgument)

				if auth.mintCalled {
					t.Fatal("MintToken() called the application service for invalid TTL")
				}

				return
			}

			if err != nil {
				t.Fatalf("MintToken() error = %v", err)
			}

			if auth.options.TTL != time.Duration(tt.ttlSeconds)*time.Second {
				t.Fatalf("application TTL = %v, want %v", auth.options.TTL, time.Duration(tt.ttlSeconds)*time.Second)
			}
		})
	}
}

func assertConnectCode(t *testing.T, err error, want connect.Code) {
	t.Helper()

	if err == nil {
		t.Fatal("expected error")
	}

	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		t.Fatalf("error = %T, want *connect.Error", err)
	}

	if connectErr.Code() != want {
		t.Fatalf("connect code = %v, want %v", connectErr.Code(), want)
	}
}

func validMintTokenRequest() *devv1.MintTokenRequest {
	return &devv1.MintTokenRequest{
		Claims: &devv1.TokenClaims{
			Subject:   "user_123",
			Audiences: []string{"canterbury"},
		},
	}
}

type fakeAuthApplication struct {
	mintCalled bool
	claims     devauth.Claims
	options    devauth.MintOptions
	token      devauth.Token
	mintErr    error
}

func (auth *fakeAuthApplication) MintToken(_ context.Context, claims devauth.Claims, options devauth.MintOptions) (devauth.Token, error) {
	auth.mintCalled = true
	auth.claims = claims
	auth.options = options
	return auth.token, auth.mintErr
}

func (auth *fakeAuthApplication) VerificationKey() devauth.VerificationKey {
	return devauth.VerificationKey{}
}
