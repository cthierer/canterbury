package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"connectrpc.com/grpchealth"
	"connectrpc.com/grpcreflect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/cthierer/canterbury/gen/go/canterbury/vault/v1/vaultv1connect"
	vaultconnect "github.com/cthierer/canterbury/internal/interfaces/connectrpc"
)

const (
	defaultAddress         = "127.0.0.1:50051"
	shutdownGracePeriod    = 10 * time.Second
	readHeaderTimeout      = 5 * time.Second
	vaultServiceAddressEnv = "VAULT_SERVICE_ADDR"
)

func main() {
	if err := run(); err != nil {
		slog.Error("vault service stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	address := configValue(vaultServiceAddressEnv, defaultAddress)
	mux := http.NewServeMux()

	vaultService := vaultconnect.NewVaultServiceHandler()
	vaultPath, vaultHandler := vaultv1connect.NewVaultServiceHandler(vaultService)
	mux.Handle(vaultPath, vaultHandler)

	checker := grpchealth.NewStaticChecker(vaultv1connect.VaultServiceName)
	healthPath, healthHandler := grpchealth.NewHandler(checker)
	mux.Handle(healthPath, healthHandler)

	reflector := grpcreflect.NewStaticReflector(vaultv1connect.VaultServiceName)
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
		slog.Info("starting vault service", "address", address)
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
