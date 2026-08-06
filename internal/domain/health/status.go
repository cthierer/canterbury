package health

// Status describes whether a service is ready to accept requests.
type Status uint8

const (
	// StatusUnknown indicates that readiness could not be determined.
	StatusUnknown Status = 0
	// StatusServing indicates that the service is ready to accept requests.
	StatusServing Status = 1
	// StatusNotServing indicates that the service is running but not ready to
	// accept requests.
	StatusNotServing Status = 2
)
