package cliapp

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"strings"
)

// Handler runs a command with its command-specific arguments.
type Handler func(ctx context.Context, args []string, output io.Writer) error

// Command describes a command exposed by an Application.
type Command struct {
	Name    string
	Summary string
	Run     Handler
}

// Application dispatches a suite-style service CLI.
type Application struct {
	Name           string
	DefaultCommand string
	Commands       []Command
	Prepare        func() error
	Footer         string
}

// Run dispatches the requested command and returns a process exit code.
func (app Application) Run(ctx context.Context, args []string, output io.Writer) int {
	commandName, commandArgs, help := app.command(args)
	if help {
		app.WriteUsage(output)
		return 0
	}

	command, ok := app.find(commandName)
	if !ok {
		slog.ErrorContext(ctx, "parse CLI command", "err", fmt.Errorf("unknown command %q", commandName))
		return 1
	}

	if app.Prepare != nil {
		if err := app.Prepare(); err != nil {
			slog.ErrorContext(ctx, "prepare CLI application", "err", err)
			return 1
		}
	}

	if command.Run == nil {
		slog.ErrorContext(ctx, "run CLI command", "err", fmt.Errorf("command %q has no handler", command.Name))
		return 1
	}

	if err := command.Run(ctx, commandArgs, output); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}

		slog.ErrorContext(ctx, "run CLI command", "command", command.Name, "err", err)
		return 1
	}

	return 0
}

// WriteUsage writes the top-level command list.
func (app Application) WriteUsage(output io.Writer) {
	_, _ = fmt.Fprintf(output, "Usage:\n\t%s [command]\n\nCommands:\n", app.Name)
	for _, command := range app.Commands {
		_, _ = fmt.Fprintf(output, "\t%-12s %s\n", command.Name, command.Summary)
	}
	if app.Footer != "" {
		_, _ = fmt.Fprintf(output, "\n%s\n", app.Footer)
	}
}

func (app Application) command(args []string) (string, []string, bool) {
	if len(args) == 0 {
		return app.DefaultCommand, nil, false
	}

	name := strings.ToLower(strings.TrimSpace(args[0]))
	if name == "-h" || name == "--help" || name == "help" {
		return "", nil, true
	}

	return name, args[1:], false
}

func (app Application) find(name string) (Command, bool) {
	for _, command := range app.Commands {
		if command.Name == name {
			return command, true
		}
	}

	return Command{}, false
}
