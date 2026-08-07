package healthgrpc_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/grpchealth"
	"github.com/cthierer/canterbury/internal/adapters/healthgrpc"
	"github.com/cthierer/canterbury/internal/domain/health"
)

const testService = "canterbury.test.v1.TestService"

func TestNewCheckerValidatesInputs(t *testing.T) {
	t.Run("requires client", func(t *testing.T) {
		_, err := healthgrpc.NewChecker(nil, testService)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("requires service", func(t *testing.T) {
		client := grpchealth.NewClient(http.DefaultClient, "http://127.0.0.1")
		_, err := healthgrpc.NewChecker(client, " \t")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestCheckerMapsGRPCHealthStatuses(t *testing.T) {
	tests := []struct {
		name   string
		remote grpchealth.Status
		want   health.Status
	}{
		{name: "serving", remote: grpchealth.StatusServing, want: health.StatusServing},
		{name: "not serving", remote: grpchealth.StatusNotServing, want: health.StatusNotServing},
		{name: "unknown", remote: grpchealth.StatusUnknown, want: health.StatusUnknown},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var requestedService string
			client := newHealthClient(t, checkerFunc(func(_ context.Context, req *grpchealth.CheckRequest) (*grpchealth.CheckResponse, error) {
				requestedService = req.Service
				return &grpchealth.CheckResponse{Status: test.remote}, nil
			}))
			checker, err := healthgrpc.NewChecker(client, " "+testService+" ")
			if err != nil {
				t.Fatalf("NewChecker() error = %v", err)
			}

			status, err := checker.Check(t.Context())
			if err != nil {
				t.Fatalf("Check() error = %v", err)
			}
			if status != test.want {
				t.Fatalf("Check() status = %v, want %v", status, test.want)
			}
			if requestedService != testService {
				t.Fatalf("requested service = %q, want %q", requestedService, testService)
			}
		})
	}
}

func TestCheckerReturnsUnknownForRemoteErrors(t *testing.T) {
	remoteErr := errors.New("health unavailable")
	client := newHealthClient(t, checkerFunc(func(context.Context, *grpchealth.CheckRequest) (*grpchealth.CheckResponse, error) {
		return nil, remoteErr
	}))
	checker, err := healthgrpc.NewChecker(client, testService)
	if err != nil {
		t.Fatalf("NewChecker() error = %v", err)
	}

	status, err := checker.Check(t.Context())
	if err == nil {
		t.Fatal("expected error")
	}
	if status != health.StatusUnknown {
		t.Fatalf("Check() status = %v, want %v", status, health.StatusUnknown)
	}
	if !strings.Contains(err.Error(), testService) {
		t.Fatalf("Check() error = %q, want service name", err)
	}
}

func TestCheckerRejectsUnsupportedStatus(t *testing.T) {
	client := newHealthClient(t, checkerFunc(func(context.Context, *grpchealth.CheckRequest) (*grpchealth.CheckResponse, error) {
		return &grpchealth.CheckResponse{Status: grpchealth.Status(99)}, nil
	}))
	checker, err := healthgrpc.NewChecker(client, testService)
	if err != nil {
		t.Fatalf("NewChecker() error = %v", err)
	}

	status, err := checker.Check(t.Context())
	if err == nil {
		t.Fatal("expected error")
	}
	if status != health.StatusUnknown {
		t.Fatalf("Check() status = %v, want %v", status, health.StatusUnknown)
	}
}

type checkerFunc func(context.Context, *grpchealth.CheckRequest) (*grpchealth.CheckResponse, error)

func (check checkerFunc) Check(ctx context.Context, req *grpchealth.CheckRequest) (*grpchealth.CheckResponse, error) {
	return check(ctx, req)
}

func newHealthClient(t *testing.T, checker grpchealth.Checker) *grpchealth.Client {
	t.Helper()

	path, handler := grpchealth.NewHandler(checker)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return grpchealth.NewClient(server.Client(), server.URL)
}
