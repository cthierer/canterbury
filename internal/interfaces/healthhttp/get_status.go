package healthhttp

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cthierer/canterbury/internal/domain/health"
)

func (service *healthServiceHandler) getStatus(ctx context.Context) (status, error) {
	result, err := service.health.Check(ctx)
	if err != nil {
		return status{}, fmt.Errorf("check service health: %w", err)
	}

	if result.Err != nil {
		slog.WarnContext(ctx, "encountered errors checking service health", "err", result.Err)
	}

	switch result.Status {
	case health.StatusNotServing:
		return status{statusNotServing}, nil
	case health.StatusServing:
		return status{statusServing}, nil
	default:
		return status{statusUnknown}, nil
	}
}
