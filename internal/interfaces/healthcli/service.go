package healthcli

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cthierer/canterbury/internal/domain/health"
)

// HealthApplication defines the health use case queried by the CLI service.
type HealthApplication interface {
	Check(ctx context.Context) (health.Result, error)
}

// HealthService adapts application health results to the CLI's serving
// decision.
type HealthService struct {
	app HealthApplication
}

// NewService creates a CLI health service backed by the given health
// application.
func NewService(app HealthApplication) (*HealthService, error) {
	if app == nil {
		return nil, fmt.Errorf("health application must not be nil")
	}

	return &HealthService{app}, nil
}

// Serving reports whether the application health result indicates that the
// service is serving. Checker and diagnostic errors are logged and produce a
// false result.
func (service *HealthService) Serving(ctx context.Context) bool {
	result, err := service.app.Check(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "checking service health", "err", err)
		return false
	}

	if result.Err != nil {
		slog.WarnContext(ctx, "checking service health", "err", result.Err)
	}

	return result.Status == health.StatusServing
}
