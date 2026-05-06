package auditfs

import (
	"context"
	"fmt"

	"github.com/cthierer/canterbury/internal/domain/audit"
)

// Record appends event as one JSON Lines record in the event date's audit file.
func (r *Recorder) Record(ctx context.Context, event audit.Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	data, err := eventDataFromDomain(event)
	if err != nil {
		return fmt.Errorf("convert event to data: %w", err)
	}

	file, err := r.openAuditFile(data.OccurredAt)
	if err != nil {
		return fmt.Errorf("open audit file for event: %w", err)
	}

	err = appendJSONL(file, data)
	if err != nil {
		_ = file.Close()
		return fmt.Errorf("append event to file: %w", err)
	}

	err = file.Close()
	if err != nil {
		return fmt.Errorf("close audit log file: %w", err)
	}

	return nil
}
