package cliapp_test

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"

	"github.com/cthierer/canterbury/internal/cliapp"
)

func TestApplicationDispatchesCommands(t *testing.T) {
	var gotArgs []string
	app := testApplication(func(_ context.Context, args []string, _ io.Writer) error {
		gotArgs = args
		return nil
	})

	if got := app.Run(t.Context(), nil, &bytes.Buffer{}); got != 0 {
		t.Fatalf("Run() default exit code = %d, want 0", got)
	}
	if gotArgs != nil {
		t.Fatalf("default command args = %#v, want nil", gotArgs)
	}

	if got := app.Run(t.Context(), []string{" HEALTHCHECK ", "--timeout", "1s"}, &bytes.Buffer{}); got != 0 {
		t.Fatalf("Run() explicit exit code = %d, want 0", got)
	}
	if len(gotArgs) != 2 || gotArgs[0] != "--timeout" || gotArgs[1] != "1s" {
		t.Fatalf("explicit command args = %#v", gotArgs)
	}
}

func TestApplicationHandlesHelpAndFailures(t *testing.T) {
	app := testApplication(func(context.Context, []string, io.Writer) error {
		return errors.New("failed")
	})

	var output bytes.Buffer
	if got := app.Run(t.Context(), []string{"help"}, &output); got != 0 {
		t.Fatalf("Run() help exit code = %d, want 0", got)
	}
	if !strings.Contains(output.String(), "test-service [command]") || !strings.Contains(output.String(), "healthcheck") {
		t.Fatalf("help output = %q", output.String())
	}

	if got := app.Run(t.Context(), []string{"missing"}, &bytes.Buffer{}); got != 1 {
		t.Fatalf("Run() unknown exit code = %d, want 1", got)
	}
	if got := app.Run(t.Context(), []string{"healthcheck"}, &bytes.Buffer{}); got != 1 {
		t.Fatalf("Run() failure exit code = %d, want 1", got)
	}
}

func TestApplicationTreatsCommandHelpAsSuccess(t *testing.T) {
	app := testApplication(func(context.Context, []string, io.Writer) error {
		return flag.ErrHelp
	})

	if got := app.Run(t.Context(), []string{"healthcheck", "--help"}, &bytes.Buffer{}); got != 0 {
		t.Fatalf("Run() exit code = %d, want 0", got)
	}
}

func TestApplicationRunsPrepareBeforeCommand(t *testing.T) {
	prepared := false
	app := testApplication(func(context.Context, []string, io.Writer) error {
		if !prepared {
			t.Fatal("handler ran before Prepare")
		}
		return nil
	})
	app.Prepare = func() error {
		prepared = true
		return nil
	}

	if got := app.Run(t.Context(), nil, &bytes.Buffer{}); got != 0 {
		t.Fatalf("Run() exit code = %d, want 0", got)
	}

	app.Prepare = func() error { return errors.New("prepare failed") }
	if got := app.Run(t.Context(), nil, &bytes.Buffer{}); got != 1 {
		t.Fatalf("Run() prepare failure exit code = %d, want 1", got)
	}
}

func testApplication(handler cliapp.Handler) cliapp.Application {
	return cliapp.Application{
		Name:           "test-service",
		DefaultCommand: "serve",
		Commands: []cliapp.Command{
			{Name: "serve", Summary: "Start service", Run: handler},
			{Name: "healthcheck", Summary: "Check service", Run: handler},
		},
		Footer: `Run "test-service healthcheck --help" for healthcheck flags.`,
	}
}
