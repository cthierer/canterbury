package connectrpc

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"connectrpc.com/connect"
)

func TestLogConnectError(t *testing.T) {
	tests := []struct {
		name string
		code connect.Code
		want slog.Level
	}{
		{
			name: "logs invalid argument at debug",
			code: connect.CodeInvalidArgument,
			want: slog.LevelDebug,
		},
		{
			name: "logs not found at debug",
			code: connect.CodeNotFound,
			want: slog.LevelDebug,
		},
		{
			name: "logs permission denied at debug",
			code: connect.CodePermissionDenied,
			want: slog.LevelDebug,
		},
		{
			name: "logs canceled at debug",
			code: connect.CodeCanceled,
			want: slog.LevelDebug,
		},
		{
			name: "logs deadline exceeded at debug",
			code: connect.CodeDeadlineExceeded,
			want: slog.LevelDebug,
		},
		{
			name: "logs unavailable at error",
			code: connect.CodeUnavailable,
			want: slog.LevelError,
		},
		{
			name: "logs unknown at error",
			code: connect.CodeUnknown,
			want: slog.LevelError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := &captureLogHandler{}
			defaultLogger := slog.Default()
			slog.SetDefault(slog.New(handler))
			t.Cleanup(func() {
				slog.SetDefault(defaultLogger)
			})

			logConnectError(
				context.Background(),
				"test log",
				errors.New("original"),
				connect.NewError(test.code, errors.New("classified")),
			)

			if len(handler.records) != 1 {
				t.Fatalf("got %d log records, want 1", len(handler.records))
			}

			if handler.records[0].Level != test.want {
				t.Fatalf("got level %s, want %s", handler.records[0].Level, test.want)
			}
		})
	}
}

type captureLogHandler struct {
	records []slog.Record
}

func (h *captureLogHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *captureLogHandler) Handle(_ context.Context, record slog.Record) error {
	h.records = append(h.records, record.Clone())
	return nil
}

func (h *captureLogHandler) WithAttrs([]slog.Attr) slog.Handler {
	return h
}

func (h *captureLogHandler) WithGroup(string) slog.Handler {
	return h
}
