package healthhttp

type statusValue string

const (
	statusUnknown    statusValue = "UNKNOWN"
	statusServing    statusValue = "SERVING"
	statusNotServing statusValue = "NOT_SERVING"
)

type status struct {
	Status statusValue `json:"status,omitempty"`
}
