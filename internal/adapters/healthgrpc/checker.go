package healthgrpc

import (
	"context"
	"fmt"
	"strings"

	"connectrpc.com/grpchealth"
	"github.com/cthierer/canterbury/internal/domain/health"
)

var _ health.Checker = (*Checker)(nil)

// Checker queries the health of a gRPC service and returns its status.
type Checker struct {
	client  *grpchealth.Client
	service string
}

// NewChecker creates a checker for a named service exposed through the gRPC
// health protocol.
func NewChecker(client *grpchealth.Client, service string) (*Checker, error) {
	if client == nil {
		return nil, fmt.Errorf("client must not be nil")
	}

	service = strings.TrimSpace(service)
	if service == "" {
		return nil, fmt.Errorf("service must not be blank")
	}

	return &Checker{client, service}, nil
}

// Check queries the remote service and maps its gRPC health status to the
// Canterbury health domain.
func (checker *Checker) Check(ctx context.Context) (health.Status, error) {
	response, err := checker.client.Check(ctx, &grpchealth.CheckRequest{Service: checker.service})
	if err != nil {
		return health.StatusUnknown, fmt.Errorf("query %q health: %w", checker.service, err)
	}

	switch response.Status {
	case grpchealth.StatusNotServing:
		return health.StatusNotServing, nil
	case grpchealth.StatusServing:
		return health.StatusServing, nil
	case grpchealth.StatusUnknown:
		return health.StatusUnknown, nil
	default:
		return health.StatusUnknown, fmt.Errorf("query %q health: unsupported status: %v", checker.service, response.Status)
	}
}
