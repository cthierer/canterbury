package vaultrpc

import (
	"strings"
	"testing"
)

func TestNewAuditContextInterceptor(t *testing.T) {
	t.Run("rejects short HMAC key", func(t *testing.T) {
		_, err := NewAuditContextInterceptor([]byte("short"))
		if err == nil {
			t.Fatal("expected error")
		}

		if !strings.Contains(err.Error(), "audit HMAC key must be at least 32 bytes") {
			t.Fatalf("got error %q, want minimum key length message", err)
		}
	})

	t.Run("copies HMAC key", func(t *testing.T) {
		key := []byte("0123456789abcdef0123456789abcdef")

		got, err := NewAuditContextInterceptor(key)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		interceptor, ok := got.(*auditContextInterceptor)
		if !ok {
			t.Fatalf("got %T, want *auditContextInterceptor", got)
		}

		key[0] = 'x'
		if string(interceptor.hmacKey) != "0123456789abcdef0123456789abcdef" {
			t.Fatalf("interceptor HMAC key was mutated through caller slice")
		}
	})
}

func TestHashRemoteAddress(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")

	t.Run("normalizes address and removes port", func(t *testing.T) {
		withPort, err := hashRemoteAddress("127.0.0.1:50051", key)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		withoutPort, err := hashRemoteAddress("127.0.0.1", key)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if withPort != withoutPort {
			t.Fatalf("hash with port = %q, hash without port = %q", withPort, withoutPort)
		}

		if !strings.HasPrefix(withPort, "hmac-sha256:") {
			t.Fatalf("got hash %q, want algorithm prefix", withPort)
		}
	})

	t.Run("normalizes IPv6 address", func(t *testing.T) {
		bracketed, err := hashRemoteAddress("[::1]:50051", key)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		raw, err := hashRemoteAddress("::1", key)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if bracketed != raw {
			t.Fatalf("bracketed IPv6 hash = %q, raw IPv6 hash = %q", bracketed, raw)
		}
	})

	t.Run("rejects invalid address", func(t *testing.T) {
		_, err := hashRemoteAddress("not an address", key)
		if err == nil {
			t.Fatal("expected error")
		}

		if !strings.Contains(err.Error(), "parse remote address") {
			t.Fatalf("got error %q, want parse remote address message", err)
		}
	})
}
