package healthhttp

import (
	"context"
	"fmt"

	"github.com/cthierer/canterbury/internal/domain/health"
)

// HealthApplication defines the readiness use case required by the HTTP handler.
type HealthApplication interface {
	Check(ctx context.Context) (health.Result, error)
}

// HealthServiceHandler exposes application readiness over HTTP.
type HealthServiceHandler struct {
	health HealthApplication
}

// NewHealthServiceHandler creates an HTTP readiness handler.
func NewHealthServiceHandler(health HealthApplication) (*HealthServiceHandler, error) {
	if health == nil {
		return nil, fmt.Errorf("health application service is required")
	}

	return &HealthServiceHandler{health}, nil
}
