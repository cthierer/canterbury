package connectrpc

import (
	"net/http"
	"testing"
)

func TestRequestIDFromHeader(t *testing.T) {
	header := http.Header{}
	header.Set(headerRequestID, " req_123 ")

	got, ok := requestIDFromHeader(header)
	if !ok {
		t.Fatal("got unset request ID")
	}

	if got != "req_123" {
		t.Fatalf("got %q, want req_123", got)
	}
}

func TestTraceIDFromHeader(t *testing.T) {
	tests := []struct {
		name string
		val  string
		want string
		ok   bool
	}{
		{
			name: "extracts and normalizes trace ID",
			val:  "00-4BF92F3577B34DA6A3CE929D0E0E4736-00f067aa0ba902b7-01",
			want: "4bf92f3577b34da6a3ce929d0e0e4736",
			ok:   true,
		},
		{
			name: "ignores missing traceparent",
		},
		{
			name: "rejects extra fields",
			val:  "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01-extra",
		},
		{
			name: "rejects forbidden version",
			val:  "ff-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
		},
		{
			name: "rejects zero trace ID",
			val:  "00-00000000000000000000000000000000-00f067aa0ba902b7-01",
		},
		{
			name: "rejects zero parent ID",
			val:  "00-4bf92f3577b34da6a3ce929d0e0e4736-0000000000000000-01",
		},
		{
			name: "rejects non-hex trace ID",
			val:  "00-zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz-00f067aa0ba902b7-01",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			header := http.Header{}
			if test.val != "" {
				header.Set(headerTraceParent, test.val)
			}

			got, ok := traceIDFromHeader(header)
			if ok != test.ok {
				t.Fatalf("ok = %v, want %v", ok, test.ok)
			}

			if got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}
