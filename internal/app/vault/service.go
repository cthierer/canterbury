package vault

import (
	"fmt"

	"github.com/cthierer/canterbury/internal/app/clock"
	domain "github.com/cthierer/canterbury/internal/domain/vault"
)

// Service coordinates controlled vault read and search use cases.
type Service struct {
	repository domain.Repository
	auditLog   AuditLogger
	clock      clock.Clock
}

// NewService creates a vault application service.
func NewService(repository domain.Repository, auditLog AuditLogger) (*Service, error) {
	if repository == nil {
		return nil, fmt.Errorf("repository must not be nil")
	}

	if auditLog == nil {
		return nil, fmt.Errorf("audit log must not be nil")
	}

	return &Service{
		repository: repository,
		auditLog:   auditLog,
		clock:      clock.SystemClock{},
	}, nil
}
