package health_test

import (
	"context"
	"errors"
	"testing"

	apphealth "github.com/cthierer/canterbury/internal/app/health"
	domainhealth "github.com/cthierer/canterbury/internal/domain/health"
)

func TestNewServiceValidatesCheckers(t *testing.T) {
	t.Run("requires a checker", func(t *testing.T) {
		_, err := apphealth.NewService()
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rejects nil checker", func(t *testing.T) {
		_, err := apphealth.NewService(nil)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("creates service", func(t *testing.T) {
		service, err := apphealth.NewService(checkerFunc(func(context.Context) (domainhealth.Status, error) {
			return domainhealth.StatusServing, nil
		}))
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		if service == nil {
			t.Fatal("expected service")
		}
	})
}

func TestServiceCheckResolvesStatuses(t *testing.T) {
	tests := []struct {
		name     string
		statuses []domainhealth.Status
		want     domainhealth.Status
	}{
		{name: "all serving", statuses: []domainhealth.Status{domainhealth.StatusServing, domainhealth.StatusServing}, want: domainhealth.StatusServing},
		{name: "unknown dominates serving", statuses: []domainhealth.Status{domainhealth.StatusServing, domainhealth.StatusUnknown}, want: domainhealth.StatusUnknown},
		{name: "not serving dominates unknown", statuses: []domainhealth.Status{domainhealth.StatusUnknown, domainhealth.StatusNotServing}, want: domainhealth.StatusNotServing},
		{name: "invalid status resolves unknown", statuses: []domainhealth.Status{domainhealth.Status(99)}, want: domainhealth.StatusUnknown},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			checkers := make([]domainhealth.Checker, 0, len(test.statuses))
			for _, status := range test.statuses {
				checkers = append(checkers, checkerFunc(func(context.Context) (domainhealth.Status, error) {
					return status, nil
				}))
			}

			service, err := apphealth.NewService(checkers...)
			if err != nil {
				t.Fatalf("NewService() error = %v", err)
			}

			result, err := service.Check(t.Context())
			if err != nil {
				t.Fatalf("Check() error = %v", err)
			}
			if result.Status != test.want {
				t.Fatalf("Check() status = %v, want %v", result.Status, test.want)
			}
		})
	}
}

func TestServiceCheckCollectsCheckerErrors(t *testing.T) {
	firstErr := errors.New("first check failed")
	secondErr := errors.New("second check failed")
	service, err := apphealth.NewService(
		checkerFunc(func(context.Context) (domainhealth.Status, error) {
			return domainhealth.StatusUnknown, firstErr
		}),
		checkerFunc(func(context.Context) (domainhealth.Status, error) {
			return domainhealth.StatusUnknown, secondErr
		}),
	)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	result, err := service.Check(t.Context())
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if result.Status != domainhealth.StatusUnknown {
		t.Fatalf("Check() status = %v, want %v", result.Status, domainhealth.StatusUnknown)
	}
	if !errors.Is(result.Err, firstErr) || !errors.Is(result.Err, secondErr) {
		t.Fatalf("Check() diagnostic error = %v, want both checker errors", result.Err)
	}
}

func TestServiceCheckShortCircuitsNotServing(t *testing.T) {
	called := false
	service, err := apphealth.NewService(
		checkerFunc(func(context.Context) (domainhealth.Status, error) {
			return domainhealth.StatusNotServing, nil
		}),
		checkerFunc(func(context.Context) (domainhealth.Status, error) {
			called = true
			return domainhealth.StatusServing, nil
		}),
	)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	result, err := service.Check(t.Context())
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if result.Status != domainhealth.StatusNotServing {
		t.Fatalf("Check() status = %v, want %v", result.Status, domainhealth.StatusNotServing)
	}
	if called {
		t.Fatal("checker after not-serving result was called")
	}
}

func TestServiceCheckHonorsCanceledContext(t *testing.T) {
	called := false
	service, err := apphealth.NewService(checkerFunc(func(context.Context) (domainhealth.Status, error) {
		called = true
		return domainhealth.StatusServing, nil
	}))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = service.Check(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Check() error = %v, want %v", err, context.Canceled)
	}
	if called {
		t.Fatal("checker was called after context cancellation")
	}
}

type checkerFunc func(context.Context) (domainhealth.Status, error)

func (check checkerFunc) Check(ctx context.Context) (domainhealth.Status, error) {
	return check(ctx)
}
