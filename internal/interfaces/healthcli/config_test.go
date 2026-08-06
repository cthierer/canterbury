package healthcli_test

import (
	"bytes"
	"errors"
	"flag"
	"strings"
	"testing"
	"time"

	"github.com/cthierer/canterbury/internal/interfaces/healthcli"
)

func TestParseConfigAppliesFlagsAndNormalization(t *testing.T) {
	cfg, err := healthcli.ParseConfig(
		[]string{"--url", " value ", "--timeout", "250ms"},
		&bytes.Buffer{},
		healthcli.Config{URL: "default", Timeout: time.Second},
		healthcli.ConfigOptions{
			CommandName: "service healthcheck",
			URLUsage:    "health endpoint URL",
			NormalizeURL: func(value string) (string, error) {
				return strings.TrimSpace(value), nil
			},
		},
	)
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}
	if cfg.URL != "value" || cfg.Timeout != 250*time.Millisecond {
		t.Fatalf("ParseConfig() = %+v", cfg)
	}
}

func TestParseConfigRejectsInvalidValues(t *testing.T) {
	options := healthcli.ConfigOptions{
		CommandName: "service healthcheck",
		URLUsage:    "health URL",
		NormalizeURL: func(string) (string, error) {
			return "", errors.New("invalid URL")
		},
	}

	if _, err := healthcli.ParseConfig(nil, &bytes.Buffer{}, healthcli.Config{Timeout: time.Second}, options); err == nil {
		t.Fatal("ParseConfig() accepted invalid URL")
	}
	options.NormalizeURL = func(value string) (string, error) { return value, nil }
	if _, err := healthcli.ParseConfig([]string{"--timeout", "0s"}, &bytes.Buffer{}, healthcli.Config{}, options); err == nil {
		t.Fatal("ParseConfig() accepted nonpositive timeout")
	}
	if _, err := healthcli.ParseConfig([]string{"unexpected"}, &bytes.Buffer{}, healthcli.Config{Timeout: time.Second}, options); err == nil {
		t.Fatal("ParseConfig() accepted unexpected argument")
	}
}

func TestParseConfigWritesHelp(t *testing.T) {
	var output bytes.Buffer
	_, err := healthcli.ParseConfig(
		[]string{"--help"},
		&output,
		healthcli.Config{},
		healthcli.ConfigOptions{CommandName: "service healthcheck", NormalizeURL: func(value string) (string, error) { return value, nil }},
	)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("ParseConfig() error = %v, want %v", err, flag.ErrHelp)
	}
	if !strings.Contains(output.String(), "service healthcheck [flags]") {
		t.Fatalf("help output = %q", output.String())
	}
}
