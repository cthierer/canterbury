package auditlog

import (
	"context"
	"fmt"

	domain "github.com/cthierer/canterbury/internal/domain/audit"
)

// RecordEvent completes missing audit envelope fields and records the event.
func (s *Service) RecordEvent(ctx context.Context, event domain.Event) error {
	if event.OccurredAt.IsZero() {
		event.OccurredAt = s.clock.Now().UTC()
	} else {
		event.OccurredAt = event.OccurredAt.UTC()
	}

	if event.ID == "" {
		id, err := s.idGenerator.NewEventID(event.OccurredAt)
		if err != nil {
			return fmt.Errorf("set event id: %w", err)
		}
		event.ID = id
	} else {
		id, err := domain.NewEventID(event.ID.String())
		if err != nil {
			return fmt.Errorf("validate event id: %w", err)
		}
		event.ID = id
	}

	if err := s.recorder.Record(ctx, event); err != nil {
		return fmt.Errorf("record event in log: %w", err)
	}

	return nil
}
