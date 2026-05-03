package auditfs

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cthierer/canterbury/internal/domain/audit"
	"github.com/cthierer/canterbury/internal/domain/vault"
)

func TestNewRecorder(t *testing.T) {
	t.Run("creates and resolves root", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "audit")

		recorder, err := NewRecorder(root)
		if err != nil {
			t.Fatalf("NewRecorder() error = %v", err)
		}

		if !filepath.IsAbs(recorder.root) {
			t.Fatalf("root = %q, want absolute path", recorder.root)
		}

		info, err := os.Stat(recorder.root)
		if err != nil {
			t.Fatalf("stat root: %v", err)
		}

		if !info.IsDir() {
			t.Fatalf("root is not a directory")
		}
	})

	t.Run("rejects empty root", func(t *testing.T) {
		_, err := NewRecorder(" ")
		if !errors.Is(err, ErrInvalidRoot) {
			t.Fatalf("NewRecorder() error = %v, want %v", err, ErrInvalidRoot)
		}
	})
}

func TestRecorderRecord(t *testing.T) {
	root := t.TempDir()
	recorder, err := NewRecorder(root)
	if err != nil {
		t.Fatalf("NewRecorder() error = %v", err)
	}

	occurredAt := time.Date(2026, 5, 3, 0, 30, 0, 123000000, time.FixedZone("east", 2*60*60))
	event := testAuditEvent("event_1", occurredAt)

	if err := recorder.Record(context.Background(), event); err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	wantPath := filepath.Join(recorder.root, "2026", "05", "2026_05_02_audit.jsonl")
	content, err := os.ReadFile(wantPath) // #nosec G304 -- test reads the recorder path it just created.
	if err != nil {
		t.Fatalf("read audit file: %v", err)
	}

	lines := strings.Split(strings.TrimSuffix(string(content), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("got %d JSONL lines, want 1: %q", len(lines), content)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &got); err != nil {
		t.Fatalf("unmarshal audit record: %v", err)
	}

	if got["id"] != "event_1" {
		t.Fatalf("id = %v, want event_1", got["id"])
	}

	if got["occurred_at"] != "2026-05-02T22:30:00.123Z" {
		t.Fatalf("occurred_at = %v, want UTC timestamp", got["occurred_at"])
	}

	outcome, ok := got["outcome"].(map[string]any)
	if !ok {
		t.Fatalf("outcome = %T, want object", got["outcome"])
	}

	if outcome["duration_ns"] != float64((1500 * time.Microsecond).Nanoseconds()) {
		t.Fatalf("duration_ns = %v, want %d", outcome["duration_ns"], (1500 * time.Microsecond).Nanoseconds())
	}

	details, ok := got["details"].(map[string]any)
	if !ok {
		t.Fatalf("details = %T, want object", got["details"])
	}

	if details["note_path"] != "Projects/Canterbury.md" {
		t.Fatalf("details.note_path = %v, want Projects/Canterbury.md", details["note_path"])
	}
}

func TestRecorderRecordAppends(t *testing.T) {
	root := t.TempDir()
	recorder, err := NewRecorder(root)
	if err != nil {
		t.Fatalf("NewRecorder() error = %v", err)
	}

	occurredAt := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	if err := recorder.Record(context.Background(), testAuditEvent("event_1", occurredAt)); err != nil {
		t.Fatalf("Record() first error = %v", err)
	}

	if err := recorder.Record(context.Background(), testAuditEvent("event_2", occurredAt)); err != nil {
		t.Fatalf("Record() second error = %v", err)
	}

	content, err := os.ReadFile(filepath.Join(recorder.root, "2026", "05", "2026_05_03_audit.jsonl"))
	if err != nil {
		t.Fatalf("read audit file: %v", err)
	}

	lines := strings.Split(strings.TrimSuffix(string(content), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d JSONL lines, want 2: %q", len(lines), content)
	}
}

func TestRecorderRecordReturnsContextError(t *testing.T) {
	recorder, err := NewRecorder(t.TempDir())
	if err != nil {
		t.Fatalf("NewRecorder() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = recorder.Record(ctx, testAuditEvent("event_1", time.Now()))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Record() error = %v, want %v", err, context.Canceled)
	}
}

func TestEventDataFromDomainValidation(t *testing.T) {
	validEvent := testAuditEvent("event_1", time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC))

	tests := []struct {
		name    string
		event   audit.Event
		wantErr error
	}{
		{
			name:    "missing ID",
			event:   func() audit.Event { event := validEvent; event.ID = ""; return event }(),
			wantErr: ErrEventMissingID,
		},
		{
			name:    "missing timestamp",
			event:   func() audit.Event { event := validEvent; event.OccurredAt = time.Time{}; return event }(),
			wantErr: ErrEventMissingTimestamp,
		},
		{
			name:    "unknown type",
			event:   func() audit.Event { event := validEvent; event.Details = nil; return event }(),
			wantErr: ErrEventUnknownType,
		},
		{
			name: "invalid actor scope",
			event: func() audit.Event {
				event := validEvent
				event.Actor.Scopes = []vault.Scope{""}
				return event
			}(),
			wantErr: ErrInvalidScopes,
		},
		{
			name: "invalid policy scope",
			event: func() audit.Event {
				event := validEvent
				event.Policy.MatchedScopes = []vault.Scope{""}
				return event
			}(),
			wantErr: ErrInvalidScopes,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := eventDataFromDomain(tt.event)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("eventDataFromDomain() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func testAuditEvent(id string, occurredAt time.Time) audit.Event {
	return audit.Event{
		ID:         audit.EventID(id),
		OccurredAt: occurredAt,
		RequestID:  audit.RequestID("request_1"),
		TraceID:    audit.TraceID("trace_1"),
		Actor: audit.Actor{
			Issuer:      "https://auth.example.test",
			SubjectHash: "sha256:subject",
			Scopes:      []vault.Scope{"personal-agent"},
		},
		Client: audit.Client{
			Interface:         audit.ClientInterfaceConnectRPC,
			UserAgent:         "test-agent",
			RemoteAddressHash: "sha256:remote",
		},
		Policy: audit.Policy{
			MappingChecksum: "sha256:mapping",
			MatchedScopes:   []vault.Scope{"personal-agent"},
			Decision:        audit.PolicyDecisionAllow,
		},
		Outcome: audit.Outcome{
			Status:   audit.OutcomeStatusSuccess,
			Code:     "ok",
			Duration: 1500 * time.Microsecond,
		},
		Details: testDetails{
			NotePath: "Projects/Canterbury.md",
		},
	}
}

type testDetails struct {
	NotePath string `json:"note_path"`
}

func (d testDetails) EventType() audit.EventType {
	return audit.EventTypeVaultReadAllowed
}
