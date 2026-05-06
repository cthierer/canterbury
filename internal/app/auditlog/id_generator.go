package auditlog

import (
	"fmt"
	"time"

	"github.com/cthierer/canterbury/internal/app/idgen"
	domain "github.com/cthierer/canterbury/internal/domain/audit"
)

// IDGenerator creates audit event identifiers from event timestamps.
type IDGenerator interface {
	NewEventID(timestamp time.Time) (domain.EventID, error)
}

type ulidGenerator struct{}

func (ulidGenerator) NewEventID(timestamp time.Time) (domain.EventID, error) {
	ulid, err := idgen.NewULID(timestamp)
	if err != nil {
		return "", fmt.Errorf("make ulid for event: %w", err)
	}

	eventID, err := domain.NewEventID(ulid)
	if err != nil {
		return "", fmt.Errorf("make event ID from ulid: %w", err)
	}

	return eventID, nil
}
