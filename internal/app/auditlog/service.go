package auditlog

import (
	"fmt"

	"github.com/cthierer/canterbury/internal/app/clock"
	domain "github.com/cthierer/canterbury/internal/domain/audit"
)

// Service coordinates audit event recording.
type Service struct {
	clock       clock.Clock
	idGenerator IDGenerator
	recorder    domain.Recorder
}

// NewService creates an audit log application service.
func NewService(recorder domain.Recorder) (*Service, error) {
	if recorder == nil {
		return nil, fmt.Errorf("recorder must not be nil")
	}

	return &Service{
		clock:       clock.SystemClock{},
		idGenerator: ulidGenerator{},
		recorder:    recorder,
	}, nil
}
