package healthcli_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cthierer/canterbury/internal/domain/health"
	"github.com/cthierer/canterbury/internal/interfaces/healthcli"
)

func TestNewServiceValidatesApplication(t *testing.T) {
	if _, err := healthcli.NewService(nil); err == nil {
		t.Fatal("NewService() error = nil, want error")
	}

	service, err := healthcli.NewService(&fakeHealthApplication{})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if service == nil {
		t.Fatal("NewService() service = nil, want service")
	}
}

func TestServingReportsApplicationStatus(t *testing.T) {
	tests := []struct {
		name   string
		status health.Status
		want   bool
	}{
		{name: "serving", status: health.StatusServing, want: true},
		{name: "not serving", status: health.StatusNotServing, want: false},
		{name: "unknown", status: health.StatusUnknown, want: false},
		{name: "invalid", status: health.Status(99), want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := mustService(t, &fakeHealthApplication{result: health.Result{Status: test.status}})

			if got := service.Serving(t.Context()); got != test.want {
				t.Fatalf("Serving() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestServingReturnsFalseForApplicationErrors(t *testing.T) {
	service := mustService(t, &fakeHealthApplication{err: errors.New("health unavailable")})

	if service.Serving(t.Context()) {
		t.Fatal("Serving() = true, want false")
	}
}

func TestServingPreservesServingStatusWithDiagnosticError(t *testing.T) {
	service := mustService(t, &fakeHealthApplication{
		result: health.Result{
			Status: health.StatusServing,
			Err:    errors.New("diagnostic warning"),
		},
	})

	if !service.Serving(t.Context()) {
		t.Fatal("Serving() = false, want true")
	}
}

func TestServingPassesContextToApplication(t *testing.T) {
	type contextKey struct{}
	value := "request-context"
	ctx := context.WithValue(t.Context(), contextKey{}, value)
	application := &fakeHealthApplication{result: health.Result{Status: health.StatusServing}}
	service := mustService(t, application)

	if !service.Serving(ctx) {
		t.Fatal("Serving() = false, want true")
	}
	if got := application.context.Value(contextKey{}); got != value {
		t.Fatalf("application context value = %v, want %v", got, value)
	}
}

func TestCheckReportsServingAndFailure(t *testing.T) {
	serving := mustService(t, &fakeHealthApplication{result: health.Result{Status: health.StatusServing}})
	if err := serving.Check(t.Context(), time.Second); err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	notServing := mustService(t, &fakeHealthApplication{result: health.Result{Status: health.StatusNotServing}})
	if err := notServing.Check(t.Context(), time.Second); !errors.Is(err, healthcli.ErrHealthcheckFailed) {
		t.Fatalf("Check() error = %v, want %v", err, healthcli.ErrHealthcheckFailed)
	}

	if err := serving.Check(t.Context(), 0); err == nil {
		t.Fatal("Check() accepted nonpositive timeout")
	}
}

func TestCheckAppliesTimeout(t *testing.T) {
	service := mustService(t, healthApplicationFunc(func(ctx context.Context) (health.Result, error) {
		<-ctx.Done()
		return health.Result{}, ctx.Err()
	}))

	if err := service.Check(t.Context(), time.Millisecond); !errors.Is(err, healthcli.ErrHealthcheckFailed) {
		t.Fatalf("Check() error = %v, want %v", err, healthcli.ErrHealthcheckFailed)
	}
}

type fakeHealthApplication struct {
	result  health.Result
	err     error
	context context.Context
}

type healthApplicationFunc func(context.Context) (health.Result, error)

func (application healthApplicationFunc) Check(ctx context.Context) (health.Result, error) {
	return application(ctx)
}

func (application *fakeHealthApplication) Check(ctx context.Context) (health.Result, error) {
	application.context = ctx
	return application.result, application.err
}

func mustService(t *testing.T, application healthcli.HealthApplication) *healthcli.HealthService {
	t.Helper()

	service, err := healthcli.NewService(application)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service
}
