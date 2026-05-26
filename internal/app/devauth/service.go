package devauth

import (
	"fmt"

	appclock "github.com/cthierer/canterbury/internal/app/clock"
	domain "github.com/cthierer/canterbury/internal/domain/devauth"
)

// Service coordinates development auth token minting.
type Service struct {
	minter domain.Minter
	clock  appclock.Clock
}

// ServiceOption customizes a development auth service.
type ServiceOption func(*Service) error

// NewService creates a development auth application service.
func NewService(minter domain.Minter, options ...ServiceOption) (*Service, error) {
	if minter == nil {
		return nil, fmt.Errorf("minter must not be nil")
	}

	service := &Service{
		minter: minter,
		clock:  appclock.SystemClock{},
	}

	for _, option := range options {
		if err := option(service); err != nil {
			return nil, err
		}
	}

	return service, nil
}

// WithClock replaces the service clock, primarily for deterministic tests.
func WithClock(clock appclock.Clock) ServiceOption {
	return func(service *Service) error {
		if clock == nil {
			return fmt.Errorf("clock must not be nil")
		}

		service.clock = clock
		return nil
	}
}
