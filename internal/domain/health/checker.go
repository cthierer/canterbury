package health

import "context"

// Checker reports the readiness of a service or dependency.
type Checker interface {
	Check(ctx context.Context) (Status, error)
}
