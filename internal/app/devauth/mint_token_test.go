package devauth

import (
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

	token, err := service.MintToken(domain.Claims{
		Subject:   " user_123 ",
		Audiences: []string{" https://canterbury.example.test "},
	})
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

	wantOptions := domain.MintOptions{
		IssuedAt:  now,
		ExpiresAt: now.Add(defaultTTL),
	}
	if minter.options != wantOptions {
		t.Fatalf("minter options = %#v, want %#v", minter.options, wantOptions)
	}
}

func TestMintTokenUsesTTL(t *testing.T) {
	now := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	minter := &recordingMinter{}
	service := newTestService(t, minter, fixedClock{now: now})

	_, err := service.MintToken(
		domain.Claims{Subject: "user_123", Audiences: []string{"canterbury"}},
		WithTTL(30*time.Minute),
	)
	if err != nil {
		t.Fatalf("MintToken() error = %v", err)
	}

	if minter.options.ExpiresAt != now.Add(30*time.Minute) {
		t.Fatalf("expires at = %v, want %v", minter.options.ExpiresAt, now.Add(30*time.Minute))
	}
}

func TestMintTokenRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name    string
		claims  domain.Claims
		options []MintOption
		wantErr error
	}{
		{
			name:    "missing subject",
			claims:  domain.Claims{Audiences: []string{"canterbury"}},
			wantErr: ErrMissingSubject,
		},
		{
			name:    "missing audience",
			claims:  domain.Claims{Subject: "user_123"},
			wantErr: ErrMissingAudience,
		},
		{
			name:    "blank audience",
			claims:  domain.Claims{Subject: "user_123", Audiences: []string{" "}},
			wantErr: ErrMissingAudience,
		},
		{
			name:    "negative ttl",
			claims:  domain.Claims{Subject: "user_123", Audiences: []string{"canterbury"}},
			options: []MintOption{WithTTL(-1 * time.Second)},
			wantErr: ErrNegativeTTL,
		},
		{
			name:    "large ttl",
			claims:  domain.Claims{Subject: "user_123", Audiences: []string{"canterbury"}},
			options: []MintOption{WithTTL(maxTTL + time.Second)},
			wantErr: ErrLargeTTL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := newTestService(t, &recordingMinter{}, fixedClock{now: time.Now()})

			_, err := service.MintToken(tt.claims, tt.options...)
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
	options         domain.MintOptions
	token           domain.Token
	verificationKey domain.VerificationKey
}

func (minter *recordingMinter) MintToken(claims domain.Claims, options domain.MintOptions) (domain.Token, error) {
	minter.claims = claims
	minter.options = options
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
