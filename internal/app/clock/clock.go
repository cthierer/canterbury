package clock

import "time"

// Clock provides the current time to application services.
type Clock interface {
	Now() time.Time
}
