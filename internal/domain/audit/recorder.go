package audit

import "context"

// Recorder stores audit events outside the protected vault content.
type Recorder interface {
	Record(ctx context.Context, event Event) error
}
