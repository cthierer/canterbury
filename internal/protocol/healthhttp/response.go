package healthhttp

const (
	// StatusUnknown indicates that health could not be determined.
	StatusUnknown = "UNKNOWN"
	// StatusServing indicates that the service is ready to accept requests.
	StatusServing = "SERVING"
	// StatusNotServing indicates that the service is not ready to accept requests.
	StatusNotServing = "NOT_SERVING"
)

// HealthResponse is the JSON representation of a Canterbury health status.
type HealthResponse struct {
	Status string `json:"status"`
}
