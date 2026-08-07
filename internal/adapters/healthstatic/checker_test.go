package healthstatic_test

import (
	"context"
	"errors"
	"testing"

	"github.com/cthierer/canterbury/internal/adapters/healthstatic"
	"github.com/cthierer/canterbury/internal/domain/health"
)

func TestCheckerLifecycle(t *testing.T) {
	checker := healthstatic.NewChecker()

	status, err := checker.Check(t.Context())
	if err != nil {
		t.Fatalf("initial Check() error = %v", err)
	}
	if status != health.StatusServing {
		t.Fatalf("initial Check() status = %v, want %v", status, health.StatusServing)
	}

	checker.SetNotServing()
	status, err = checker.Check(t.Context())
	if err != nil {
		t.Fatalf("Check() after SetNotServing() error = %v", err)
	}
	if status != health.StatusNotServing {
		t.Fatalf("Check() after SetNotServing() status = %v, want %v", status, health.StatusNotServing)
	}
}

func TestCheckerHonorsCanceledContext(t *testing.T) {
	checker := healthstatic.NewChecker()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	status, err := checker.Check(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Check() error = %v, want %v", err, context.Canceled)
	}
	if status != health.StatusServing {
		t.Fatalf("Check() status = %v, want current status %v", status, health.StatusServing)
	}
}
