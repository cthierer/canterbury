package healthhttp

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cthierer/canterbury/internal/domain/health"
	healthprotocol "github.com/cthierer/canterbury/internal/protocol/healthhttp"
)

func (service *healthServiceHandler) getHealth(ctx context.Context) (status health.Status, response healthprotocol.HealthResponse, err error) {
	result, err := service.health.Check(ctx)
	if err != nil {
		return health.StatusUnknown, healthprotocol.HealthResponse{}, fmt.Errorf("check service health: %w", err)
	}

	if result.Err != nil {
		slog.WarnContext(ctx, "encountered errors checking service health", "err", result.Err)
	}

	return result.Status, healthResponseFromDomain(result.Status), nil
}

func healthResponseFromDomain(domainStatus health.Status) healthprotocol.HealthResponse {
	switch domainStatus {
	case health.StatusNotServing:
		return healthprotocol.HealthResponse{Status: healthprotocol.StatusNotServing}
	case health.StatusServing:
		return healthprotocol.HealthResponse{Status: healthprotocol.StatusServing}
	default:
		return healthprotocol.HealthResponse{Status: healthprotocol.StatusUnknown}
	}
}
