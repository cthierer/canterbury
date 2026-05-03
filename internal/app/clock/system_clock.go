package clock

import "time"

// SystemClock provides time from the host system clock.
type SystemClock struct{}

// Now returns the current host system time.
func (SystemClock) Now() time.Time {
	return time.Now()
}
