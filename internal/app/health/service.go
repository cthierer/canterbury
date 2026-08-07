package health

import (
	"context"
	"errors"
	"fmt"

	domain "github.com/cthierer/canterbury/internal/domain/health"
)

// Service resolves the aggregate readiness of ordered health checkers.
type Service struct {
	checkers []domain.Checker
}

// NewService creates a health service that resolves the status of one or more
// ordered checkers.
func NewService(checkers ...domain.Checker) (*Service, error) {
	if len(checkers) < 1 {
		return nil, fmt.Errorf("there must be at least 1 checker")
	}

	for _, checker := range checkers {
		if checker == nil {
			return nil, fmt.Errorf("checker must not be nil")
		}
	}

	return &Service{checkers}, nil
}

// Check resolves checker statuses in order. A not-serving result short-circuits
// the remaining checks because it cannot be superseded by another status.
func (service *Service) Check(ctx context.Context) (domain.Result, error) {
	resolved := domain.StatusServing
	var errs []error

	for _, checker := range service.checkers {
		if err := ctx.Err(); err != nil {
			return domain.Result{}, err
		}

		status, err := checker.Check(ctx)
		if err != nil {
			resolved = resolve(resolved, domain.StatusUnknown)
			errs = append(errs, err)
			continue
		}

		resolved = resolve(resolved, status)
		if resolved == domain.StatusNotServing {
			break
		}
	}

	return domain.Result{Status: resolved, Err: errors.Join(errs...)}, nil
}

// resolve determines the appropriate status given the current status and the
// next status. NotServing always takes precedence, followed by Unknown, then Serving.
// Serving can only be returned if both statuses are Serving.
func resolve(status domain.Status, next domain.Status) domain.Status {
	if status == domain.StatusNotServing || next == domain.StatusNotServing {
		return domain.StatusNotServing
	}

	if status == domain.StatusServing && next == domain.StatusServing {
		return domain.StatusServing
	}

	return domain.StatusUnknown
}
