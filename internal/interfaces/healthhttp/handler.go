package healthhttp

import (
	"context"
	"fmt"
	"net/http"

	"github.com/cthierer/canterbury/internal/domain/health"
)

const (
	livePath  = "/live"
	readyPath = "/ready"
)

// HealthApplication defines the health use case required by the HTTP handlers.
type HealthApplication interface {
	Check(ctx context.Context) (health.Result, error)
}

// NewHandler creates an HTTP handler that serves separate liveness and
// readiness endpoints.
func NewHandler(liveChecker HealthApplication, readyChecker HealthApplication) (http.Handler, error) {
	liveHandler, err := newHealthServiceHandler(liveChecker)
	if err != nil {
		return nil, fmt.Errorf("initialize handler for live check: %w", err)
	}

	readyHandler, err := newHealthServiceHandler(readyChecker)
	if err != nil {
		return nil, fmt.Errorf("initialize handler for ready check: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle(livePath, liveHandler)
	mux.Handle(readyPath, readyHandler)

	return mux, nil
}
