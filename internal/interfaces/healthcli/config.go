package healthcli

import (
	"flag"
	"fmt"
	"io"
	"time"
)

// Config controls a healthcheck command.
type Config struct {
	URL     string
	Timeout time.Duration
}

// ConfigOptions customizes the shared healthcheck flag parser.
type ConfigOptions struct {
	CommandName  string
	URLUsage     string
	NormalizeURL func(string) (string, error)
}

// ParseConfig applies command flags and validates a healthcheck configuration.
func ParseConfig(args []string, output io.Writer, cfg Config, options ConfigOptions) (Config, error) {
	flags := flag.NewFlagSet(options.CommandName, flag.ContinueOnError)
	flags.SetOutput(output)
	flags.StringVar(&cfg.URL, "url", cfg.URL, options.URLUsage)
	flags.DurationVar(&cfg.Timeout, "timeout", cfg.Timeout, "healthcheck timeout")
	flags.Usage = func() {
		writeUsage(output, flags, options.CommandName)
	}

	if err := flags.Parse(args); err != nil {
		return Config{}, err
	}
	if flags.NArg() > 0 {
		return Config{}, fmt.Errorf("unexpected healthcheck argument %q", flags.Arg(0))
	}
	if options.NormalizeURL == nil {
		return Config{}, fmt.Errorf("health URL normalizer is required")
	}

	url, err := options.NormalizeURL(cfg.URL)
	if err != nil {
		return Config{}, fmt.Errorf("validate health URL: %w", err)
	}
	cfg.URL = url

	if cfg.Timeout <= 0 {
		return Config{}, fmt.Errorf("healthcheck timeout must be positive")
	}

	return cfg, nil
}

func writeUsage(output io.Writer, flags *flag.FlagSet, commandName string) {
	_, _ = fmt.Fprintf(output, "Usage:\n\t%s [flags]\n\nFlags:\n", commandName)
	flags.PrintDefaults()
}
