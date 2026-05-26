package vaultrpc

import (
	"testing"
)

func TestNewVaultServiceHandler(t *testing.T) {
	t.Run("requires vault application", func(t *testing.T) {
		_, err := NewVaultServiceHandler(nil)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("creates handler", func(t *testing.T) {
		handler, err := NewVaultServiceHandler(&fakeVaultApplication{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if handler == nil {
			t.Fatal("expected handler")
		}
	})
}
