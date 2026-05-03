package audit

import (
	"errors"
	"strings"
	"testing"
)

func TestNewEventID(t *testing.T) {
	t.Run("trims value", func(t *testing.T) {
		got, err := NewEventID(" event_123 ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got != "event_123" {
			t.Fatalf("got %q, want event_123", got)
		}
	})

	t.Run("rejects empty value", func(t *testing.T) {
		_, err := NewEventID(" ")
		if !errors.Is(err, ErrInvalidEventID) {
			t.Fatalf("got error %v, want %v", err, ErrInvalidEventID)
		}
	})

	t.Run("rejects long value", func(t *testing.T) {
		_, err := NewEventID(strings.Repeat("a", maxIDLength+1))
		if !errors.Is(err, ErrInvalidEventID) {
			t.Fatalf("got error %v, want %v", err, ErrInvalidEventID)
		}
	})
}

func TestNewRequestID(t *testing.T) {
	t.Run("trims value", func(t *testing.T) {
		got, err := NewRequestID(" req_123 ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got != "req_123" {
			t.Fatalf("got %q, want req_123", got)
		}
	})

	t.Run("rejects empty value", func(t *testing.T) {
		_, err := NewRequestID(" ")
		if !errors.Is(err, ErrInvalidRequestID) {
			t.Fatalf("got error %v, want %v", err, ErrInvalidRequestID)
		}
	})

	t.Run("rejects long value", func(t *testing.T) {
		_, err := NewRequestID(strings.Repeat("a", maxIDLength+1))
		if !errors.Is(err, ErrInvalidRequestID) {
			t.Fatalf("got error %v, want %v", err, ErrInvalidRequestID)
		}
	})
}

func TestNewTraceID(t *testing.T) {
	t.Run("allows empty value", func(t *testing.T) {
		got, err := NewTraceID(" ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got != "" {
			t.Fatalf("got %q, want empty trace ID", got)
		}
	})

	t.Run("trims value", func(t *testing.T) {
		got, err := NewTraceID(" trace_123 ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got != "trace_123" {
			t.Fatalf("got %q, want trace_123", got)
		}
	})

	t.Run("rejects long value", func(t *testing.T) {
		_, err := NewTraceID(strings.Repeat("a", maxIDLength+1))
		if !errors.Is(err, ErrInvalidTraceID) {
			t.Fatalf("got error %v, want %v", err, ErrInvalidTraceID)
		}
	})
}

func TestEventType(t *testing.T) {
	t.Run("returns unknown for missing details", func(t *testing.T) {
		got := Event{}.Type()
		if got != EventTypeUnknown {
			t.Fatalf("got %q, want %q", got, EventTypeUnknown)
		}
	})

	t.Run("returns details event type", func(t *testing.T) {
		event := Event{Details: fakeDetails{eventType: EventTypeVaultReadAllowed}}

		got := event.Type()
		if got != EventTypeVaultReadAllowed {
			t.Fatalf("got %q, want %q", got, EventTypeVaultReadAllowed)
		}
	})
}

type fakeDetails struct {
	eventType EventType
}

func (d fakeDetails) EventType() EventType {
	return d.eventType
}
