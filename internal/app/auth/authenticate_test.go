package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/cthierer/canterbury/internal/domain/vault"
)

func TestNewAuthenticator(t *testing.T) {
	scopeMapper := newTestScopeMapper(t)
	verifier := fakeTokenVerifier{}

	authenticator, err := NewAuthenticator(" https://auth.example.test ", " https://canterbury.example.test ", scopeMapper, verifier)
	if err != nil {
		t.Fatalf("NewAuthenticator() error = %v", err)
	}

	if authenticator.issuer != "https://auth.example.test" {
		t.Fatalf("issuer = %q, want trimmed issuer", authenticator.issuer)
	}

	if authenticator.audience != "https://canterbury.example.test" {
		t.Fatalf("audience = %q, want trimmed audience", authenticator.audience)
	}
}

func TestNewAuthenticatorRejectsInvalidConfig(t *testing.T) {
	scopeMapper := newTestScopeMapper(t)
	verifier := fakeTokenVerifier{}

	tests := []struct {
		name        string
		issuer      string
		audience    string
		scopeMapper *ScopeMapper
		verifier    TokenVerifier
		want        string
	}{
		{
			name:        "blank issuer",
			issuer:      " ",
			audience:    "https://canterbury.example.test",
			scopeMapper: scopeMapper,
			verifier:    verifier,
			want:        "issuer is required",
		},
		{
			name:        "blank audience",
			issuer:      "https://auth.example.test",
			audience:    " ",
			scopeMapper: scopeMapper,
			verifier:    verifier,
			want:        "audience is required",
		},
		{
			name:        "nil scope mapper",
			issuer:      "https://auth.example.test",
			audience:    "https://canterbury.example.test",
			scopeMapper: nil,
			verifier:    verifier,
			want:        "scope mapper is required",
		},
		{
			name:        "nil verifier",
			issuer:      "https://auth.example.test",
			audience:    "https://canterbury.example.test",
			scopeMapper: scopeMapper,
			verifier:    nil,
			want:        "verifier is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewAuthenticator(test.issuer, test.audience, test.scopeMapper, test.verifier)
			if err == nil {
				t.Fatal("expected error")
			}

			if err.Error() != test.want {
				t.Fatalf("NewAuthenticator() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestAuthenticatorAuthenticate(t *testing.T) {
	authenticator := newTestAuthenticator(t, fakeTokenVerifier{
		claims: TokenClaims{
			Issuer:  " https://auth.example.test ",
			Subject: " user_123 ",
		},
	})

	got, err := authenticator.Authenticate(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}

	if got.Issuer != "https://auth.example.test" {
		t.Fatalf("Issuer = %q, want auth issuer", got.Issuer)
	}

	if got.Subject != "user_123" {
		t.Fatalf("Subject = %q, want user_123", got.Subject)
	}

	if got.SubjectHash != testSubjectHash("https://auth.example.test", "user_123") {
		t.Fatalf("SubjectHash = %q, want stable hash", got.SubjectHash)
	}

	wantScopes := []vault.Scope{"personal-agent", "public-site"}
	if !reflect.DeepEqual(got.Scopes, wantScopes) {
		t.Fatalf("Scopes = %#v, want %#v", got.Scopes, wantScopes)
	}

	if got.MappingChecksum != "sha256:test" {
		t.Fatalf("MappingChecksum = %q, want sha256:test", got.MappingChecksum)
	}
}

func TestAuthenticatorAuthenticatePassesTokenRequirements(t *testing.T) {
	verifier := &recordingTokenVerifier{
		claims: TokenClaims{
			Issuer:  "https://auth.example.test",
			Subject: "user_123",
		},
	}
	authenticator := newTestAuthenticator(t, verifier)

	_, err := authenticator.Authenticate(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}

	if verifier.token != "test-token" {
		t.Fatalf("token = %q, want test-token", verifier.token)
	}

	wantRequirements := TokenRequirements{
		Issuer:   "https://auth.example.test",
		Audience: "https://canterbury.example.test",
	}
	if verifier.requirements != wantRequirements {
		t.Fatalf("requirements = %#v, want %#v", verifier.requirements, wantRequirements)
	}
}

func TestAuthenticatorAuthenticateRejectsUnknownSubject(t *testing.T) {
	authenticator := newTestAuthenticator(t, fakeTokenVerifier{
		claims: TokenClaims{
			Issuer:  "https://auth.example.test",
			Subject: "missing_user",
		},
	})

	_, err := authenticator.Authenticate(context.Background(), "test-token")
	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(err, ErrPrincipalResolutionFailed) {
		t.Fatalf("Authenticate() error = %v, want %v", err, ErrPrincipalResolutionFailed)
	}

	failureContext, ok := FailureContextFromError(err)
	if !ok {
		t.Fatal("expected failure context")
	}

	if failureContext.Issuer != "https://auth.example.test" {
		t.Fatalf("failure issuer = %q, want auth issuer", failureContext.Issuer)
	}

	if failureContext.SubjectHash != testSubjectHash("https://auth.example.test", "missing_user") {
		t.Fatalf("failure subject hash = %q, want stable hash", failureContext.SubjectHash)
	}
}

func TestAuthenticatorAuthenticateRejectsMissingSubject(t *testing.T) {
	authenticator := newTestAuthenticator(t, fakeTokenVerifier{
		claims: TokenClaims{
			Issuer:  "https://auth.example.test",
			Subject: " ",
		},
	})

	_, err := authenticator.Authenticate(context.Background(), "test-token")
	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(err, ErrMissingSubject) {
		t.Fatalf("Authenticate() error = %v, want %v", err, ErrMissingSubject)
	}

	failureContext, ok := FailureContextFromError(err)
	if !ok {
		t.Fatal("expected failure context")
	}

	if failureContext.Issuer != "https://auth.example.test" {
		t.Fatalf("failure issuer = %q, want auth issuer", failureContext.Issuer)
	}

	if failureContext.SubjectHash != "" {
		t.Fatalf("failure subject hash = %q, want empty", failureContext.SubjectHash)
	}
}

func TestAuthenticatorAuthenticateReturnsVerifierErrors(t *testing.T) {
	verifierErr := errors.New("bad token")
	authenticator := newTestAuthenticator(t, fakeTokenVerifier{err: verifierErr})

	_, err := authenticator.Authenticate(context.Background(), "test-token")
	if !errors.Is(err, verifierErr) {
		t.Fatalf("Authenticate() error = %v, want %v", err, verifierErr)
	}
}

func TestAuthenticatorAuthenticateHonorsCanceledContext(t *testing.T) {
	authenticator := newTestAuthenticator(t, fakeTokenVerifier{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := authenticator.Authenticate(ctx, "test-token")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Authenticate() error = %v, want %v", err, context.Canceled)
	}
}

func newTestAuthenticator(t *testing.T, verifier TokenVerifier) *Authenticator {
	t.Helper()

	authenticator, err := NewAuthenticator(
		"https://auth.example.test",
		"https://canterbury.example.test",
		newTestScopeMapper(t),
		verifier,
	)
	if err != nil {
		t.Fatalf("NewAuthenticator() error = %v", err)
	}

	return authenticator
}

func testSubjectHash(issuer string, subject string) string {
	sum := sha256.Sum256([]byte(issuer + "\x00" + subject))
	return "sha256:" + hex.EncodeToString(sum[:])
}

type fakeTokenVerifier struct {
	claims TokenClaims
	err    error
}

func (verifier fakeTokenVerifier) VerifyToken(ctx context.Context, _ string, _ TokenRequirements) (TokenClaims, error) {
	if err := ctx.Err(); err != nil {
		return TokenClaims{}, err
	}

	if verifier.err != nil {
		return TokenClaims{}, fmt.Errorf("fake verify token: %w", verifier.err)
	}

	return verifier.claims, nil
}

type recordingTokenVerifier struct {
	token        string
	requirements TokenRequirements
	claims       TokenClaims
}

func (verifier *recordingTokenVerifier) VerifyToken(ctx context.Context, token string, requirements TokenRequirements) (TokenClaims, error) {
	if err := ctx.Err(); err != nil {
		return TokenClaims{}, err
	}

	verifier.token = token
	verifier.requirements = requirements

	return verifier.claims, nil
}
