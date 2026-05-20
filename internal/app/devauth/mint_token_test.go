package devauth

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	domain "github.com/cthierer/canterbury/internal/domain/devauth"
)

func TestMintTokenNormalizesClaimsAndUsesDefaultTTL(t *testing.T) {
	now := time.Date(2026, 5, 12, 9, 30, 0, 0, time.UTC)
	minter := &recordingMinter{
		token: domain.Token{JWT: "token", Type: domain.TokenTypeBearer, ExpiresAt: now.Add(defaultTTL)},
	}
	service := newTestService(t, minter, fixedClock{now: now})

	token, err := service.MintToken(
		t.Context(),
		domain.Claims{
			Subject:   " user_123 ",
			Audiences: []string{" https://canterbury.example.test "},
		},
		domain.MintOptions{},
	)
	if err != nil {
		t.Fatalf("MintToken() error = %v", err)
	}

	if token.JWT != "token" {
		t.Fatalf("MintToken() token JWT = %q, want %q", token.JWT, "token")
	}

	wantClaims := domain.Claims{
		Subject:   "user_123",
		Audiences: []string{"https://canterbury.example.test"},
	}
	if !reflect.DeepEqual(minter.claims, wantClaims) {
		t.Fatalf("minter claims = %#v, want %#v", minter.claims, wantClaims)
	}

	if minter.issuedAt != now {
		t.Fatalf("issued at = %v, want %v", minter.issuedAt, now)
	}
	if minter.expiresAt != now.Add(defaultTTL) {
		t.Fatalf("expires at = %v, want %v", minter.expiresAt, now.Add(defaultTTL))
	}
}

func TestMintTokenUsesTTL(t *testing.T) {
	now := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	minter := &recordingMinter{}
	service := newTestService(t, minter, fixedClock{now: now})

	_, err := service.MintToken(
		t.Context(),
		domain.Claims{Subject: "user_123", Audiences: []string{"canterbury"}},
		domain.MintOptions{
			TTL: 30 * time.Minute,
		},
	)
	if err != nil {
		t.Fatalf("MintToken() error = %v", err)
	}

	if minter.expiresAt != now.Add(30*time.Minute) {
		t.Fatalf("expires at = %v, want %v", minter.expiresAt, now.Add(30*time.Minute))
	}
}

func TestMintTokenRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name    string
		claims  domain.Claims
		options domain.MintOptions
		wantErr error
	}{
		{
			name:    "missing subject",
			claims:  domain.Claims{Audiences: []string{"canterbury"}},
			wantErr: domain.ErrMissingSubject,
		},
		{
			name:    "missing audience",
			claims:  domain.Claims{Subject: "user_123"},
			wantErr: domain.ErrMissingAudience,
		},
		{
			name:    "blank audience",
			claims:  domain.Claims{Subject: "user_123", Audiences: []string{" "}},
			wantErr: domain.ErrMissingAudience,
		},
		{
			name:   "negative ttl",
			claims: domain.Claims{Subject: "user_123", Audiences: []string{"canterbury"}},
			options: domain.MintOptions{
				TTL: -1 * time.Second,
			},
			wantErr: domain.ErrNegativeTTL,
		},
		{
			name:   "large ttl",
			claims: domain.Claims{Subject: "user_123", Audiences: []string{"canterbury"}},
			options: domain.MintOptions{
				TTL: maxTTL + time.Second,
			},
			wantErr: domain.ErrLargeTTL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := newTestService(t, &recordingMinter{}, fixedClock{now: time.Now()})

			_, err := service.MintToken(t.Context(), tt.claims, tt.options)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("MintToken() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestVerificationKeyReturnsMinterKey(t *testing.T) {
	want := domain.VerificationKey{ID: "key-1", Algorithm: domain.SigningAlgorithmEdDSA}
	service := newTestService(t, &recordingMinter{verificationKey: want}, fixedClock{now: time.Now()})

	got := service.VerificationKey()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("VerificationKey() = %#v, want %#v", got, want)
	}
}

func newTestService(t *testing.T, minter domain.Minter, clock fixedClock) *Service {
	t.Helper()

	service, err := NewService(minter, WithClock(clock))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	return service
}

type recordingMinter struct {
	claims          domain.Claims
	issuedAt        time.Time
	expiresAt       time.Time
	token           domain.Token
	verificationKey domain.VerificationKey
}

func (minter *recordingMinter) MintToken(_ context.Context, claims domain.Claims, issuedAt time.Time, expiresAt time.Time) (domain.Token, error) {
	minter.claims = claims
	minter.issuedAt = issuedAt
	minter.expiresAt = expiresAt
	return minter.token, nil
}

func (minter *recordingMinter) VerificationKey() domain.VerificationKey {
	return minter.verificationKey
}

type fixedClock struct {
	now time.Time
}

func (clock fixedClock) Now() time.Time {
	return clock.now
}
