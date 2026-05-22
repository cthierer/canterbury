// Package main starts the Canterbury development auth service.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"connectrpc.com/grpchealth"
	"connectrpc.com/grpcreflect"
	"github.com/cthierer/canterbury/gen/go/canterbury/dev/v1/devv1connect"
	"github.com/cthierer/canterbury/internal/adapters/devauthjwt"
	"github.com/cthierer/canterbury/internal/app/devauth"
	"github.com/cthierer/canterbury/internal/interfaces/devrpc"
	"github.com/cthierer/canterbury/internal/interfaces/keyshttp"
	"github.com/joho/godotenv"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

const (
	defaultAddress      = "127.0.0.1:50052"
	defaultIssuer       = "devauth.canterbury.local"
	devAuthAddressEnv   = "DEV_AUTH_ADDR"
	devAuthIssuerEnv    = "DEV_AUTH_ISSUER"
	readHeaderTimeout   = 5 * time.Second
	shutdownGracePeriod = 10 * time.Second
)

func main() {
	if err := run(); err != nil {
		slog.ErrorContext(context.Background(), "program failed", "err", err)
		os.Exit(1)
	}
}

func run() error {
	return runArgs(os.Args[1:], os.Stdout)
}

func runArgs(args []string, output io.Writer) error {
	cmd, err := parseCommand(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			writeUsage(output)
			return nil
		}

		return fmt.Errorf("parse CLI command: %w", err)
	}

	switch cmd {
	case commandServe:
		if err := loadLocalEnv(); err != nil {
			return err
		}

		cfg, err := loadServeConfig(args[1:], output)
		if err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return nil
			}

			return fmt.Errorf("parse serve configuration: %w", err)
		}

		err = serve(cfg)
		if err != nil {
			return fmt.Errorf("start server: %w", err)
		}
	default:
		return fmt.Errorf("unsupported command %q", cmd)
	}

	return nil
}

type command string

const (
	commandServe command = "serve"
)

var (
	errUnknownCommand = errors.New("unknown command")
)

func parseCommand(args []string) (command, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("no arguments passed")
	}

	commandString := strings.TrimSpace(args[0])

	switch strings.ToLower(commandString) {
	case "-h", "--help", "help":
		return "", flag.ErrHelp
	case "serve":
		return commandServe, nil
	}

	return "", fmt.Errorf("parsing command %q: %w", commandString, errUnknownCommand)
}

type serveConfig struct {
	Address string
	Issuer  string
}

func loadServeConfig(args []string, output io.Writer) (serveConfig, error) {
	cfg := serveConfig{
		Address: configValue(devAuthAddressEnv, defaultAddress),
		Issuer:  configValue(devAuthIssuerEnv, defaultIssuer),
	}

	flags := flag.NewFlagSet("dev-auth serve", flag.ContinueOnError)
	flags.SetOutput(output)
	flags.StringVar(&cfg.Address, "addr", cfg.Address, "HTTP listen address")
	flags.StringVar(&cfg.Issuer, "issuer", cfg.Issuer, "issuer claim for minted JWTs")
	flags.Usage = func() {
		writeServeUsage(output, flags)
	}

	if err := flags.Parse(args); err != nil {
		return serveConfig{}, err
	}

	if flags.NArg() > 0 {
		return serveConfig{}, fmt.Errorf("unexpected serve argument %q", flags.Arg(0))
	}

	if strings.TrimSpace(cfg.Address) == "" {
		return serveConfig{}, fmt.Errorf("address must not be empty")
	}

	return cfg, nil
}

func serve(cfg serveConfig) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mux := http.NewServeMux()

	minter, err := devauthjwt.NewMinter(cfg.Issuer)
	if err != nil {
		return fmt.Errorf("initialize minter: %w", err)
	}

	authApplication, err := devauth.NewService(minter)
	if err != nil {
		return fmt.Errorf("initialize app service: %w", err)
	}

	authService, err := devrpc.NewDevAuthServiceHandler(authApplication)
	if err != nil {
		return fmt.Errorf("initialize auth connect service: %w", err)
	}

	authPath, authHandler := devv1connect.NewDevAuthServiceHandler(authService)
	mux.Handle(authPath, authHandler)

	jwksHandler, err := keyshttp.NewKeyStoreServiceHandler(authApplication)
	if err != nil {
		return fmt.Errorf("initialize JWKS service: %w", err)
	}
	mux.Handle("/.well-known/jwks.json", jwksHandler)

	checker := grpchealth.NewStaticChecker(devv1connect.DevAuthServiceName)
	healthPath, healthHandler := grpchealth.NewHandler(checker)
	mux.Handle(healthPath, healthHandler)

	reflector := grpcreflect.NewStaticReflector(devv1connect.DevAuthServiceName)
	reflectV1Path, reflectV1Handler := grpcreflect.NewHandlerV1(reflector)
	mux.Handle(reflectV1Path, reflectV1Handler)
	reflectV1AlphaPath, reflectV1AlphaHandler := grpcreflect.NewHandlerV1Alpha(reflector)
	mux.Handle(reflectV1AlphaPath, reflectV1AlphaHandler)

	server := &http.Server{
		Addr:              cfg.Address,
		Handler:           h2c.NewHandler(mux, &http2.Server{}),
		ReadHeaderTimeout: readHeaderTimeout,
	}

	errs := make(chan error, 1)
	go func() {
		slog.InfoContext(ctx, "starting devauth service")
		errs <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}

		return nil
	case err := <-errs:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}

		return err
	}
}

func configValue(name string, fallback string) string {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}

	return value
}

func loadLocalEnv() error {
	if err := godotenv.Load(".env"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("load dotenv configuration: %w", err)
	}

	return nil
}

func writeUsage(output io.Writer) {
	_, _ = fmt.Fprint(output, `Usage:
  dev-auth serve [flags]

Commands:
  serve    Start the development auth service

Run "dev-auth serve --help" for serve flags.
`)
}

func writeServeUsage(output io.Writer, flags *flag.FlagSet) {
	_, _ = fmt.Fprint(output, `Usage:
  dev-auth serve [flags]

Flags:
`)
	flags.PrintDefaults()
}
