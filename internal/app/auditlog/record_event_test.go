package auditlog

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "github.com/cthierer/canterbury/internal/domain/audit"
)

func TestNewService(t *testing.T) {
	t.Run("rejects missing recorder", func(t *testing.T) {
		_, err := NewService(nil)
		if err == nil {
			t.Fatal("got nil error, want error")
		}
	})

	t.Run("creates service", func(t *testing.T) {
		service, err := NewService(&recordingRecorder{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if service == nil {
			t.Fatal("got nil service")
		}
	})
}

func TestRecordEvent(t *testing.T) {
	t.Run("fills missing ID and timestamp", func(t *testing.T) {
		now := time.Date(2026, time.May, 3, 14, 25, 43, 123000000, time.FixedZone("EDT", -4*60*60))
		wantTimestamp := now.UTC()
		idGenerator := &recordingIDGenerator{id: "generated_id"}
		recorder := &recordingRecorder{}
		service := &Service{
			clock:       fixedClock{now: now},
			idGenerator: idGenerator,
			recorder:    recorder,
		}

		err := service.RecordEvent(context.Background(), domain.Event{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if idGenerator.calls != 1 {
			t.Fatalf("ID generator calls = %d, want 1", idGenerator.calls)
		}

		if !idGenerator.timestamp.Equal(wantTimestamp) {
			t.Fatalf("ID generator timestamp = %v, want %v", idGenerator.timestamp, wantTimestamp)
		}

		if recorder.calls != 1 {
			t.Fatalf("recorder calls = %d, want 1", recorder.calls)
		}

		if recorder.event.ID != "generated_id" {
			t.Fatalf("recorded ID = %q, want generated_id", recorder.event.ID)
		}

		if !recorder.event.OccurredAt.Equal(wantTimestamp) {
			t.Fatalf("recorded timestamp = %v, want %v", recorder.event.OccurredAt, wantTimestamp)
		}

		if recorder.event.OccurredAt.Location() != time.UTC {
			t.Fatalf("recorded timestamp location = %v, want UTC", recorder.event.OccurredAt.Location())
		}
	})

	t.Run("normalizes provided ID and timestamp", func(t *testing.T) {
		occurredAt := time.Date(2026, time.May, 3, 14, 25, 43, 123000000, time.FixedZone("EDT", -4*60*60))
		wantTimestamp := occurredAt.UTC()
		idGenerator := &recordingIDGenerator{id: "unused_id"}
		recorder := &recordingRecorder{}
		service := &Service{
			clock:       fixedClock{now: time.UnixMilli(0)},
			idGenerator: idGenerator,
			recorder:    recorder,
		}

		err := service.RecordEvent(context.Background(), domain.Event{
			ID:         " event_123 ",
			OccurredAt: occurredAt,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if idGenerator.calls != 0 {
			t.Fatalf("ID generator calls = %d, want 0", idGenerator.calls)
		}

		if recorder.event.ID != "event_123" {
			t.Fatalf("recorded ID = %q, want event_123", recorder.event.ID)
		}

		if !recorder.event.OccurredAt.Equal(wantTimestamp) {
			t.Fatalf("recorded timestamp = %v, want %v", recorder.event.OccurredAt, wantTimestamp)
		}

		if recorder.event.OccurredAt.Location() != time.UTC {
			t.Fatalf("recorded timestamp location = %v, want UTC", recorder.event.OccurredAt.Location())
		}
	})

	t.Run("rejects invalid provided ID", func(t *testing.T) {
		recorder := &recordingRecorder{}
		service := &Service{
			clock:       fixedClock{now: time.UnixMilli(0)},
			idGenerator: &recordingIDGenerator{id: "unused_id"},
			recorder:    recorder,
		}

		err := service.RecordEvent(context.Background(), domain.Event{ID: " "})
		if !errors.Is(err, domain.ErrInvalidEventID) {
			t.Fatalf("got error %v, want %v", err, domain.ErrInvalidEventID)
		}

		if recorder.calls != 0 {
			t.Fatalf("recorder calls = %d, want 0", recorder.calls)
		}
	})

	t.Run("returns ID generator error", func(t *testing.T) {
		generatorErr := errors.New("generator failed")
		recorder := &recordingRecorder{}
		service := &Service{
			clock:       fixedClock{now: time.UnixMilli(0)},
			idGenerator: &recordingIDGenerator{err: generatorErr},
			recorder:    recorder,
		}

		err := service.RecordEvent(context.Background(), domain.Event{})
		if !errors.Is(err, generatorErr) {
			t.Fatalf("got error %v, want %v", err, generatorErr)
		}

		if recorder.calls != 0 {
			t.Fatalf("recorder calls = %d, want 0", recorder.calls)
		}
	})

	t.Run("returns recorder error", func(t *testing.T) {
		recorderErr := errors.New("record failed")
		service := &Service{
			clock:       fixedClock{now: time.UnixMilli(0)},
			idGenerator: &recordingIDGenerator{id: "generated_id"},
			recorder:    &recordingRecorder{err: recorderErr},
		}

		err := service.RecordEvent(context.Background(), domain.Event{})
		if !errors.Is(err, recorderErr) {
			t.Fatalf("got error %v, want %v", err, recorderErr)
		}
	})
}

type fixedClock struct {
	now time.Time
}

func (c fixedClock) Now() time.Time {
	return c.now
}

type recordingIDGenerator struct {
	id        domain.EventID
	err       error
	timestamp time.Time
	calls     int
}

func (g *recordingIDGenerator) NewEventID(timestamp time.Time) (domain.EventID, error) {
	g.calls++
	g.timestamp = timestamp
	return g.id, g.err
}

type recordingRecorder struct {
	event domain.Event
	err   error
	calls int
}

func (r *recordingRecorder) Record(_ context.Context, event domain.Event) error {
	r.calls++
	r.event = event
	return r.err
}
