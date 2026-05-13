// Package main starts the Canterbury development auth service.
package main

import (
	"context"
	"errors"
	"fmt"
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
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

const (
	defaultAddress      = "127.0.0.1:50052"
	readHeaderTimeout   = 5 * time.Second
	shutdownGracePeriod = 10 * time.Second
)

func main() {
	if err := run(); err != nil {
		slog.Error("program failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	args := os.Args[1:]

	cmd, err := parseCommand(args)
	if err != nil {
		return fmt.Errorf("parse CLI command: %w", err)
	}

	switch cmd {
	case commandServe:
		cfg := loadServeConfig(args[1:])

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
	case "serve":
		return commandServe, nil
	}

	return "", fmt.Errorf("parsing command %q: %w", commandString, errUnknownCommand)
}

type serveConfig struct {
	Address string
	Issuer  string
}

func loadServeConfig(_ []string) serveConfig {
	// TODO load service configuration from environment, CLI args
	return serveConfig{
		Issuer: "devauth.canterbury.local",
	}
}

func serve(cfg serveConfig) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mux := http.NewServeMux()

	address := cfg.Address
	if address == "" {
		address = defaultAddress
	}

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

	checker := grpchealth.NewStaticChecker(devv1connect.DevAuthServiceName)
	healthPath, healthHandler := grpchealth.NewHandler(checker)
	mux.Handle(healthPath, healthHandler)

	reflector := grpcreflect.NewStaticReflector(devv1connect.DevAuthServiceName)
	reflectV1Path, reflectV1Handler := grpcreflect.NewHandlerV1(reflector)
	mux.Handle(reflectV1Path, reflectV1Handler)
	reflectV1AlphaPath, reflectV1AlphaHandler := grpcreflect.NewHandlerV1Alpha(reflector)
	mux.Handle(reflectV1AlphaPath, reflectV1AlphaHandler)

	server := &http.Server{
		Addr:              address,
		Handler:           h2c.NewHandler(mux, &http2.Server{}),
		ReadHeaderTimeout: readHeaderTimeout,
	}

	errs := make(chan error, 1)
	go func() {
		slog.Info("starting devauth service", "address", address)
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
