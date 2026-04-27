package vault

import (
	"fmt"

	"github.com/cthierer/canterbury/internal/app/auth"
	domain "github.com/cthierer/canterbury/internal/domain/vault"
)

// Service coordinates controlled vault read and search use cases.
type Service struct {
	repository domain.Repository
	principal  auth.Principal
}

// NewService creates a vault application service.
func NewService(repository domain.Repository, principal auth.Principal) (*Service, error) {
	if repository == nil {
		return nil, fmt.Errorf("repository must not be nil")
	}

	if len(principal.Scopes) < 1 {
		return nil, fmt.Errorf("principal must have at least 1 scope")
	}

	return &Service{repository: repository, principal: principal}, nil
}
