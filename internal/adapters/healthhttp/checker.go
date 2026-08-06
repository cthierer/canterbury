package healthhttp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/cthierer/canterbury/internal/domain/health"
	healthprotocol "github.com/cthierer/canterbury/internal/protocol/healthhttp"
)

const maxHealthResponseBytes = 4 << 10

// Checker checks the health status exposed by an HTTP service.
type Checker struct {
	healthURL *url.URL
	client    *http.Client
}

// NewChecker creates a health checker for an HTTP health endpoint.
func NewChecker(healthURL *url.URL, client *http.Client) (*Checker, error) {
	if healthURL == nil {
		return nil, fmt.Errorf("health URL must not be nil")
	}

	if client == nil {
		return nil, fmt.Errorf("HTTP client must not be nil")
	}

	return &Checker{healthURL: healthURL, client: client}, nil
}

// Check requests the HTTP health endpoint and maps its response to the
// Canterbury health domain.
func (checker *Checker) Check(ctx context.Context) (health.Status, error) {
	if err := ctx.Err(); err != nil {
		return health.StatusUnknown, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checker.healthURL.String(), nil)
	if err != nil {
		return health.StatusUnknown, fmt.Errorf("build request: %w", err)
	}

	resp, err := checker.client.Do(req)
	if err != nil {
		return health.StatusUnknown, fmt.Errorf("get health over http: %w", err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxHealthResponseBytes+1))
	if err != nil {
		return health.StatusUnknown, fmt.Errorf("read health status body: %w", err)
	}

	if len(bodyBytes) > maxHealthResponseBytes {
		return health.StatusUnknown, fmt.Errorf("health response exceeds %d bytes", maxHealthResponseBytes)
	}

	body := healthprotocol.HealthResponse{}
	err = json.Unmarshal(bodyBytes, &body)
	if err != nil {
		return health.StatusUnknown, fmt.Errorf("unmarshal health status body: %w", err)
	}

	status, err := domainStatusFromResponse(body)
	if err != nil {
		return health.StatusUnknown, fmt.Errorf("determine service status: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		if status != health.StatusServing {
			return health.StatusUnknown, fmt.Errorf("HTTP 200 response has %v health status", status)
		}
	case http.StatusServiceUnavailable:
		if status == health.StatusServing {
			return health.StatusUnknown, fmt.Errorf("HTTP 503 response has serving health status")
		}
	default:
		return health.StatusUnknown, fmt.Errorf("unexpected HTTP status %d", resp.StatusCode)
	}

	return status, nil
}

func domainStatusFromResponse(response healthprotocol.HealthResponse) (health.Status, error) {
	switch response.Status {
	case healthprotocol.StatusNotServing:
		return health.StatusNotServing, nil
	case healthprotocol.StatusServing:
		return health.StatusServing, nil
	case healthprotocol.StatusUnknown:
		return health.StatusUnknown, nil
	default:
		return health.StatusUnknown, fmt.Errorf("unknown health status %q", response.Status)
	}
}
