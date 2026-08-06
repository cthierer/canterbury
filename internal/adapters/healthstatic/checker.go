package healthstatic

import (
	"context"
	"sync"

	"github.com/cthierer/canterbury/internal/domain/health"
)

var _ health.Checker = (*Checker)(nil)

// Checker is a static checker that returns a set status.
type Checker struct {
	mutex  sync.RWMutex
	status health.Status
}

// NewChecker creates a checker that initially reports serving.
func NewChecker() *Checker {
	return &Checker{status: health.StatusServing}
}

// Check returns the checker's current status.
func (checker *Checker) Check(ctx context.Context) (health.Status, error) {
	checker.mutex.RLock()
	defer checker.mutex.RUnlock()

	if err := ctx.Err(); err != nil {
		return checker.status, err
	}

	return checker.status, nil
}

// SetNotServing makes all subsequent checks report not serving.
func (checker *Checker) SetNotServing() {
	checker.mutex.Lock()
	defer checker.mutex.Unlock()

	checker.status = health.StatusNotServing
}
