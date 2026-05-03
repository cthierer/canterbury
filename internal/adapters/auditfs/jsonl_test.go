package auditfs

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/cthierer/canterbury/internal/domain/audit"
)

func TestAppendJSONLEscapesStringNewlines(t *testing.T) {
	var buffer bytes.Buffer

	event := eventData{
		ID:      "event_1",
		Details: newlineDetails{Message: "first line\nsecond line"},
	}

	if err := appendJSONL(&buffer, event); err != nil {
		t.Fatalf("appendJSONL() error = %v", err)
	}

	got := buffer.String()
	if strings.Count(got, "\n") != 1 {
		t.Fatalf("got %q, want exactly one physical newline", got)
	}

	var decoded struct {
		Details newlineDetails `json:"details"`
	}
	if err := json.Unmarshal(buffer.Bytes(), &decoded); err != nil {
		t.Fatalf("unmarshal JSONL record: %v", err)
	}

	if decoded.Details.Message != "first line\nsecond line" {
		t.Fatalf("message = %q, want original newline", decoded.Details.Message)
	}
}

func TestAppendJSONLReturnsShortWriteError(t *testing.T) {
	err := appendJSONL(shortWriter{}, eventData{ID: "event_1"})
	if !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("appendJSONL() error = %v, want %v", err, io.ErrShortWrite)
	}
}

type newlineDetails struct {
	Message string `json:"message"`
}

func (d newlineDetails) EventType() audit.EventType {
	return ""
}

type shortWriter struct{}

func (w shortWriter) Write(data []byte) (int, error) {
	return len(data) - 1, nil
}
