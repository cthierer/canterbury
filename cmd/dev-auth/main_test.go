package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/cthierer/canterbury/internal/adapters/devauthjwt"
)

func TestParseCommand(t *testing.T) {
	t.Run("accepts serve command", func(t *testing.T) {
		got, err := parseCommand([]string{" serve "})
		if err != nil {
			t.Fatalf("parseCommand() error = %v", err)
		}

		if got != commandServe {
			t.Fatalf("parseCommand() = %q, want %q", got, commandServe)
		}
	})

	t.Run("rejects missing command", func(t *testing.T) {
		_, err := parseCommand(nil)
		if err == nil {
			t.Fatal("expected error")
		}

		if !strings.Contains(err.Error(), "no arguments passed") {
			t.Fatalf("parseCommand() error = %q, want missing argument message", err)
		}
	})

	t.Run("rejects unknown command", func(t *testing.T) {
		_, err := parseCommand([]string{"mint"})
		if !errors.Is(err, errUnknownCommand) {
			t.Fatalf("parseCommand() error = %v, want %v", err, errUnknownCommand)
		}
	})
}

func TestLoadServeConfigUsesDevelopmentDefaults(t *testing.T) {
	got := loadServeConfig(nil)

	if got.Address != "" {
		t.Fatalf("address = %q, want empty value so server uses %q", got.Address, defaultAddress)
	}

	if got.Issuer != "devauth.canterbury.local" {
		t.Fatalf("issuer = %q, want development issuer", got.Issuer)
	}
}

func TestServeRejectsInvalidIssuerBeforeListening(t *testing.T) {
	err := serve(serveConfig{Issuer: " "})
	if !errors.Is(err, devauthjwt.ErrInvalidIssuer) {
		t.Fatalf("serve() error = %v, want %v", err, devauthjwt.ErrInvalidIssuer)
	}
}
