package authjwt

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cthierer/canterbury/internal/app/auth"
	"github.com/golang-jwt/jwt/v5"
)

func TestNewVerifierRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name           string
		jwksURL        string
		allowedMethods []string
		want           string
	}{
		{
			name:           "blank JWKS URL",
			jwksURL:        " ",
			allowedMethods: []string{jwt.SigningMethodHS256.Alg()},
			want:           "jwks url must not be blank",
		},
		{
			name:           "no allowed methods",
			jwksURL:        "https://auth.example.test/.well-known/jwks.json",
			allowedMethods: nil,
			want:           "at least 1 method is required",
		},
		{
			name:           "blank method",
			jwksURL:        "https://auth.example.test/.well-known/jwks.json",
			allowedMethods: []string{" "},
			want:           "method must not be blank",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewVerifier(context.Background(), test.jwksURL, test.allowedMethods)
			if err == nil {
				t.Fatal("expected error")
			}

			if err.Error() != test.want {
				t.Fatalf("NewVerifier() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestVerifierVerifyToken(t *testing.T) {
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	key := []byte("test-secret")
	verifier := newTestVerifier(now, key)
	token := signTestToken(t, key, jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    "https://auth.example.test",
		Subject:   "user_123",
		Audience:  jwt.ClaimStrings{"https://canterbury.example.test", "extra-audience"},
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		IssuedAt:  jwt.NewNumericDate(now),
	})

	got, err := verifier.VerifyToken(context.Background(), token, auth.TokenRequirements{
		Issuer:   " https://auth.example.test ",
		Audience: " https://canterbury.example.test ",
	})
	if err != nil {
		t.Fatalf("VerifyToken() error = %v", err)
	}

	if got.Issuer != "https://auth.example.test" {
		t.Fatalf("Issuer = %q, want auth issuer", got.Issuer)
	}

	if got.Subject != "user_123" {
		t.Fatalf("Subject = %q, want user_123", got.Subject)
	}

	if len(got.Audiences) != 2 || got.Audiences[0] != "https://canterbury.example.test" || got.Audiences[1] != "extra-audience" {
		t.Fatalf("Audiences = %#v, want two audiences", got.Audiences)
	}

	if !got.ExpiresAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("ExpiresAt = %v, want %v", got.ExpiresAt, now.Add(time.Hour))
	}
}

func TestVerifierVerifyTokenReturnsIndependentAudienceSlice(t *testing.T) {
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	key := []byte("test-secret")
	verifier := newTestVerifier(now, key)
	token := signTestToken(t, key, jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    "https://auth.example.test",
		Subject:   "user_123",
		Audience:  jwt.ClaimStrings{"https://canterbury.example.test"},
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
	})

	first, err := verifier.VerifyToken(context.Background(), token, auth.TokenRequirements{})
	if err != nil {
		t.Fatalf("VerifyToken() first error = %v", err)
	}

	first.Audiences[0] = "mutated"

	second, err := verifier.VerifyToken(context.Background(), token, auth.TokenRequirements{})
	if err != nil {
		t.Fatalf("VerifyToken() second error = %v", err)
	}

	if second.Audiences[0] != "https://canterbury.example.test" {
		t.Fatalf("Audiences[0] = %q, want original audience", second.Audiences[0])
	}
}

func TestVerifierVerifyTokenRejectsInvalidTokens(t *testing.T) {
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	key := []byte("test-secret")

	tests := []struct {
		name         string
		token        string
		requirements auth.TokenRequirements
		want         error
	}{
		{
			name: "wrong issuer",
			token: signTestToken(t, key, jwt.SigningMethodHS256, jwt.RegisteredClaims{
				Issuer:    "https://other.example.test",
				Subject:   "user_123",
				Audience:  jwt.ClaimStrings{"https://canterbury.example.test"},
				ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			}),
			requirements: auth.TokenRequirements{
				Issuer:   "https://auth.example.test",
				Audience: "https://canterbury.example.test",
			},
			want: auth.ErrWrongIssuer,
		},
		{
			name: "wrong audience",
			token: signTestToken(t, key, jwt.SigningMethodHS256, jwt.RegisteredClaims{
				Issuer:    "https://auth.example.test",
				Subject:   "user_123",
				Audience:  jwt.ClaimStrings{"https://other.example.test"},
				ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			}),
			requirements: auth.TokenRequirements{
				Issuer:   "https://auth.example.test",
				Audience: "https://canterbury.example.test",
			},
			want: auth.ErrWrongAudience,
		},
		{
			name: "expired token",
			token: signTestToken(t, key, jwt.SigningMethodHS256, jwt.RegisteredClaims{
				Issuer:    "https://auth.example.test",
				Subject:   "user_123",
				Audience:  jwt.ClaimStrings{"https://canterbury.example.test"},
				ExpiresAt: jwt.NewNumericDate(now.Add(-time.Hour)),
			}),
			requirements: auth.TokenRequirements{
				Issuer:   "https://auth.example.test",
				Audience: "https://canterbury.example.test",
			},
			want: auth.ErrExpiredToken,
		},
		{
			name: "missing expiration",
			token: signTestToken(t, key, jwt.SigningMethodHS256, jwt.RegisteredClaims{
				Issuer:   "https://auth.example.test",
				Subject:  "user_123",
				Audience: jwt.ClaimStrings{"https://canterbury.example.test"},
			}),
			requirements: auth.TokenRequirements{
				Issuer:   "https://auth.example.test",
				Audience: "https://canterbury.example.test",
			},
			want: auth.ErrMalformedToken,
		},
		{
			name: "disallowed method",
			token: signTestToken(t, key, jwt.SigningMethodHS512, jwt.RegisteredClaims{
				Issuer:    "https://auth.example.test",
				Subject:   "user_123",
				Audience:  jwt.ClaimStrings{"https://canterbury.example.test"},
				ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			}),
			requirements: auth.TokenRequirements{
				Issuer:   "https://auth.example.test",
				Audience: "https://canterbury.example.test",
			},
			want: auth.ErrInvalidTokenSignature,
		},
		{
			name:         "malformed token",
			token:        "not-a-jwt",
			requirements: auth.TokenRequirements{},
			want:         auth.ErrMalformedToken,
		},
	}

	verifier := newTestVerifier(now, key)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := verifier.VerifyToken(context.Background(), test.token, test.requirements)
			if err == nil {
				t.Fatal("expected error")
			}

			if !errors.Is(err, test.want) {
				t.Fatalf("VerifyToken() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestVerifierVerifyTokenHonorsCanceledContext(t *testing.T) {
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	verifier := newTestVerifier(now, []byte("test-secret"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := verifier.VerifyToken(ctx, "unused", auth.TokenRequirements{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("VerifyToken() error = %v, want %v", err, context.Canceled)
	}
}

func newTestVerifier(now time.Time, key []byte) *Verifier {
	return &Verifier{
		clock:   fixedClock{now: now},
		methods: []string{jwt.SigningMethodHS256.Alg()},
		keyFunc: func(_ *jwt.Token) (any, error) {
			return key, nil
		},
	}
}

func signTestToken(t *testing.T, key []byte, method jwt.SigningMethod, claims jwt.Claims) string {
	t.Helper()

	token, err := jwt.NewWithClaims(method, claims).SignedString(key)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	return token
}

type fixedClock struct {
	now time.Time
}

func (clock fixedClock) Now() time.Time {
	return clock.now
}
