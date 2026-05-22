package keyshttp

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cthierer/canterbury/internal/domain/devauth"
)

func TestServeHTTPReturnsJWKS(t *testing.T) {
	publicKey := ed25519.PublicKey([]byte("12345678901234567890123456789012"))
	handler := newTestHandler(t, devauth.VerificationKey{
		ID:        "dev-key",
		Algorithm: devauth.SigningAlgorithmEdDSA,
		PublicKey: publicKey,
	})
	req := httptest.NewRequest(http.MethodGet, "/.well-known/jwks.json", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/jwk-set+json" {
		t.Fatalf("content type = %q, want %q", got, "application/jwk-set+json")
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("cache control = %q, want %q", got, "no-store")
	}

	var got keySet
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal JWKS response: %v", err)
	}

	if len(got.Keys) != 1 {
		t.Fatalf("key count = %d, want 1", len(got.Keys))
	}

	gotKey := got.Keys[0]
	if gotKey.Kty != "OKP" || gotKey.Crv != "Ed25519" || gotKey.Alg != "EdDSA" || gotKey.Use != "sig" {
		t.Fatalf("JWK metadata = %#v, want Ed25519 signature key", gotKey)
	}
	if gotKey.Kid != "dev-key" {
		t.Fatalf("key ID = %q, want %q", gotKey.Kid, "dev-key")
	}
	if wantX := base64.RawURLEncoding.EncodeToString(publicKey); gotKey.X != wantX {
		t.Fatalf("key x = %q, want %q", gotKey.X, wantX)
	}
}

func TestNewKeyStoreServiceHandlerRejectsMissingApplication(t *testing.T) {
	_, err := NewKeyStoreServiceHandler(nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestServeHTTPHeadReturnsHeadersWithoutBody(t *testing.T) {
	handler := newTestHandler(t, testVerificationKey())
	req := httptest.NewRequest(http.MethodHead, "/.well-known/jwks.json", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Body.String(); got != "" {
		t.Fatalf("body = %q, want empty body", got)
	}
	if got := rec.Header().Get("Content-Length"); got == "" {
		t.Fatal("expected content length header")
	}
}

func TestServeHTTPRejectsUnsupportedMethods(t *testing.T) {
	handler := newTestHandler(t, testVerificationKey())
	req := httptest.NewRequest(http.MethodPost, "/.well-known/jwks.json", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
	if got := rec.Header().Get("Allow"); got != "GET, HEAD" {
		t.Fatalf("allow = %q, want %q", got, "GET, HEAD")
	}
}

func TestServeHTTPHidesInvalidKeyDetails(t *testing.T) {
	handler := newTestHandler(t, devauth.VerificationKey{PublicKey: "not a public key"})
	req := httptest.NewRequest(http.MethodGet, "/.well-known/jwks.json", nil)
	rec := httptest.NewRecorder()
	logHandler := captureLogger(t)

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if got := rec.Body.String(); got != "internal server error\n" {
		t.Fatalf("body = %q, want generic internal server error", got)
	}
	if strings.Contains(rec.Body.String(), "unsupported key type") {
		t.Fatalf("body = %q, want no key details", rec.Body.String())
	}
	logRecords := logHandler.snapshot()
	if len(logRecords) != 1 {
		t.Fatalf("log count = %d, want 1", len(logRecords))
	}
	if got := logRecords[0].Level; got != slog.LevelError {
		t.Fatalf("log level = %s, want %s", got, slog.LevelError)
	}
}

func TestServeHTTPHandlesCanceledAndTimedOutRequests(t *testing.T) {
	tests := []struct {
		name    string
		ctx     func() context.Context
		wantMsg string
	}{
		{
			name: "canceled request",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			wantMsg: "request canceled\n",
		},
		{
			name: "deadline exceeded",
			ctx: func() context.Context {
				return deadlineExceededContext(t)
			},
			wantMsg: "request timeout\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := newTestHandler(t, testVerificationKey())
			req := httptest.NewRequest(http.MethodGet, "/.well-known/jwks.json", nil).WithContext(test.ctx())
			rec := httptest.NewRecorder()
			logHandler := captureLogger(t)

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusRequestTimeout {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusRequestTimeout)
			}
			if got := rec.Body.String(); got != test.wantMsg {
				t.Fatalf("body = %q, want %q", got, test.wantMsg)
			}
			logRecords := logHandler.snapshot()
			if len(logRecords) != 1 {
				t.Fatalf("log count = %d, want 1", len(logRecords))
			}
			if got := logRecords[0].Level; got != slog.LevelDebug {
				t.Fatalf("log level = %s, want %s", got, slog.LevelDebug)
			}
		})
	}
}

func newTestHandler(t *testing.T, verificationKey devauth.VerificationKey) *KeyStoreServiceHandler {
	t.Helper()

	handler, err := NewKeyStoreServiceHandler(fakeKeyStore{verificationKey: verificationKey})
	if err != nil {
		t.Fatalf("NewKeyStoreServiceHandler() error = %v", err)
	}

	return handler
}

func testVerificationKey() devauth.VerificationKey {
	return devauth.VerificationKey{
		ID:        "dev-key",
		Algorithm: devauth.SigningAlgorithmEdDSA,
		PublicKey: ed25519.PublicKey([]byte("12345678901234567890123456789012")),
	}
}

type fakeKeyStore struct {
	verificationKey devauth.VerificationKey
}

func (store fakeKeyStore) VerificationKey() devauth.VerificationKey {
	return store.verificationKey
}

func captureLogger(t *testing.T) *captureLogHandler {
	t.Helper()

	handler := &captureLogHandler{}
	defaultLogger := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() {
		slog.SetDefault(defaultLogger)
	})

	return handler
}

func deadlineExceededContext(t *testing.T) context.Context {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	t.Cleanup(cancel)

	select {
	case <-ctx.Done():
		if ctx.Err() != context.DeadlineExceeded {
			t.Fatalf("context error = %v, want %v", ctx.Err(), context.DeadlineExceeded)
		}
	case <-time.After(time.Second):
		t.Fatal("context did not reach deadline exceeded state")
	}

	return ctx
}

type captureLogHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *captureLogHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *captureLogHandler) Handle(_ context.Context, record slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.records = append(h.records, record.Clone())
	return nil
}

func (h *captureLogHandler) WithAttrs([]slog.Attr) slog.Handler {
	return h
}

func (h *captureLogHandler) WithGroup(string) slog.Handler {
	return h
}

func (h *captureLogHandler) snapshot() []slog.Record {
	h.mu.Lock()
	defer h.mu.Unlock()

	records := make([]slog.Record, len(h.records))
	copy(records, h.records)
	return records
}
