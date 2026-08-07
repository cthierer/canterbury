package healthhttp

import "fmt"

// healthServiceHandler exposes an application health status over HTTP.
type healthServiceHandler struct {
	health HealthApplication
}

// newHealthServiceHandler creates an HTTP health-status handler.
func newHealthServiceHandler(health HealthApplication) (*healthServiceHandler, error) {
	if health == nil {
		return nil, fmt.Errorf("health application service is required")
	}

	return &healthServiceHandler{health}, nil
}
