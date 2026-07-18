package main

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"path/filepath"
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

	t.Run("recognizes help", func(t *testing.T) {
		_, err := parseCommand([]string{"--help"})
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("parseCommand() error = %v, want %v", err, flag.ErrHelp)
		}
	})
}

func TestLoadServeConfigUsesDevelopmentDefaults(t *testing.T) {
	unsetEnv(t, "DEV_AUTH_ADDR")
	unsetEnv(t, "DEV_AUTH_ISSUER")

	got, err := loadServeConfig(nil, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("loadServeConfig() error = %v", err)
	}

	if got.Addr != "127.0.0.1:50052" {
		t.Fatalf("addr = %q, want default address", got.Addr)
	}

	if got.Issuer != "devauth.canterbury.local" {
		t.Fatalf("issuer = %q, want default issuer", got.Issuer)
	}
}

func TestLoadServeConfigUsesEnvironmentDefaults(t *testing.T) {
	t.Setenv("DEV_AUTH_ADDR", "127.0.0.1:60052")
	t.Setenv("DEV_AUTH_ISSUER", "https://dev-auth.example.test")

	got, err := loadServeConfig(nil, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("loadServeConfig() error = %v", err)
	}

	if got.Addr != "127.0.0.1:60052" {
		t.Fatalf("addr = %q, want environment address", got.Addr)
	}
	if got.Issuer != "https://dev-auth.example.test" {
		t.Fatalf("issuer = %q, want environment issuer", got.Issuer)
	}
}

func TestLoadServeConfigFlagsOverrideEnvironment(t *testing.T) {
	t.Setenv("DEV_AUTH_ADDR", "127.0.0.1:60052")
	t.Setenv("DEV_AUTH_ISSUER", "https://environment.example.test")

	got, err := loadServeConfig([]string{
		"--addr", "127.0.0.1:70052",
		"--issuer", "https://flag.example.test",
	}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("loadServeConfig() error = %v", err)
	}

	if got.Addr != "127.0.0.1:70052" {
		t.Fatalf("addr = %q, want flag address", got.Addr)
	}
	if got.Issuer != "https://flag.example.test" {
		t.Fatalf("issuer = %q, want flag issuer", got.Issuer)
	}
}

func TestLoadServeConfigRejectsBadArguments(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "unexpected argument",
			args:    []string{"unexpected"},
			wantErr: `unexpected serve argument "unexpected"`,
		},
		{
			name:    "unknown flag",
			args:    []string{"--unknown"},
			wantErr: "flag provided but not defined",
		},
		{
			name:    "empty address",
			args:    []string{"--addr="},
			wantErr: "address must not be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadServeConfig(tt.args, &bytes.Buffer{})
			if err == nil {
				t.Fatal("expected error")
			}

			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("loadServeConfig() error = %q, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestLoadServeConfigShowsHelp(t *testing.T) {
	var output bytes.Buffer

	_, err := loadServeConfig([]string{"--help"}, &output)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("loadServeConfig() error = %v, want %v", err, flag.ErrHelp)
	}

	if !strings.Contains(output.String(), "dev-auth serve [flags]") {
		t.Fatalf("help output = %q, want serve usage", output.String())
	}
}

func TestRunArgsShowsTopLevelHelp(t *testing.T) {
	var output bytes.Buffer

	if err := runArgs([]string{"help"}, &output); err != nil {
		t.Fatalf("runArgs() error = %v", err)
	}

	if !strings.Contains(output.String(), "dev-auth serve [flags]") {
		t.Fatalf("help output = %q, want top-level usage", output.String())
	}
}

func TestServeRejectsInvalidIssuerBeforeListening(t *testing.T) {
	err := serve(serveConfig{Issuer: " "})
	if !errors.Is(err, devauthjwt.ErrInvalidIssuer) {
		t.Fatalf("serve() error = %v, want %v", err, devauthjwt.ErrInvalidIssuer)
	}
}

func TestLoadLocalEnv(t *testing.T) {
	t.Run("ignores missing dotenv file", func(t *testing.T) {
		inTempDir(t)

		if err := loadLocalEnv(); err != nil {
			t.Fatalf("loadLocalEnv() error = %v", err)
		}
	})

	t.Run("loads dotenv without overriding environment", func(t *testing.T) {
		dir := inTempDir(t)
		t.Setenv("DEV_AUTH_DOTENV_VALUE", "environment")
		unsetEnv(t, "DEV_AUTH_DOTENV_ONLY")

		err := os.WriteFile(
			filepath.Join(dir, ".env"),
			[]byte("DEV_AUTH_DOTENV_VALUE=dotenv\nDEV_AUTH_DOTENV_ONLY=loaded\n"),
			0o600,
		)
		if err != nil {
			t.Fatalf("write dotenv: %v", err)
		}

		if err := loadLocalEnv(); err != nil {
			t.Fatalf("loadLocalEnv() error = %v", err)
		}

		if got := os.Getenv("DEV_AUTH_DOTENV_VALUE"); got != "environment" {
			t.Fatalf("dotenv value = %q, want environment", got)
		}
		if got := os.Getenv("DEV_AUTH_DOTENV_ONLY"); got != "loaded" {
			t.Fatalf("dotenv only value = %q, want loaded", got)
		}
	})
}

func inTempDir(t *testing.T) string {
	t.Helper()

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}

	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("change working directory: %v", err)
	}

	t.Cleanup(func() {
		if err := os.Chdir(oldDir); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	})

	return dir
}

func unsetEnv(t *testing.T, name string) {
	t.Helper()

	value, existed := os.LookupEnv(name)
	if err := os.Unsetenv(name); err != nil {
		t.Fatalf("unset environment variable %q: %v", name, err)
	}

	t.Cleanup(func() {
		if !existed {
			if err := os.Unsetenv(name); err != nil {
				t.Fatalf("restore environment variable %q: %v", name, err)
			}

			return
		}

		if err := os.Setenv(name, value); err != nil {
			t.Fatalf("restore environment variable %q: %v", name, err)
		}
	})
}
