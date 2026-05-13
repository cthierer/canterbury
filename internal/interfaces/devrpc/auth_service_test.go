package devrpc

import (
	"context"
	"errors"
	"strings"
	"testing"

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

func TestMintTokenReturnsUnimplementedUntilWired(t *testing.T) {
	auth := &fakeAuthApplication{}
	handler, err := NewDevAuthServiceHandler(auth)
	if err != nil {
		t.Fatalf("NewDevAuthServiceHandler() error = %v", err)
	}

	_, err = handler.MintToken(t.Context(), connect.NewRequest(&devv1.MintTokenRequest{}))
	if err == nil {
		t.Fatal("expected error")
	}

	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		t.Fatalf("MintToken() error = %T, want *connect.Error", err)
	}

	if connectErr.Code() != connect.CodeUnimplemented {
		t.Fatalf("MintToken() code = %v, want %v", connectErr.Code(), connect.CodeUnimplemented)
	}

	if auth.mintCalled {
		t.Fatal("MintToken() called the application service before RPC mapping is implemented")
	}
}

type fakeAuthApplication struct {
	mintCalled bool
}

func (auth *fakeAuthApplication) MintToken(context.Context, devauth.Claims, devauth.MintOptions) (devauth.Token, error) {
	auth.mintCalled = true
	return devauth.Token{}, nil
}

func (auth *fakeAuthApplication) VerificationKey() devauth.VerificationKey {
	return devauth.VerificationKey{}
}
