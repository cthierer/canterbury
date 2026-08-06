package healthhttp_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/cthierer/canterbury/internal/adapters/healthhttp"
	"github.com/cthierer/canterbury/internal/domain/health"
	healthprotocol "github.com/cthierer/canterbury/internal/protocol/healthhttp"
)

func TestNewCheckerValidatesInputs(t *testing.T) {
	healthURL, err := url.Parse("http://127.0.0.1/health/live")
	if err != nil {
		t.Fatalf("parse health URL: %v", err)
	}

	tests := []struct {
		name      string
		healthURL *url.URL
		client    *http.Client
	}{
		{name: "requires URL", client: http.DefaultClient},
		{name: "requires client", healthURL: healthURL},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := healthhttp.NewChecker(test.healthURL, test.client); err == nil {
				t.Fatal("NewChecker() error = nil, want error")
			}
		})
	}
}

func TestCheckerMapsValidHealthResponses(t *testing.T) {
	tests := []struct {
		name       string
		code       int
		status     string
		wantStatus health.Status
	}{
		{name: "serving", code: http.StatusOK, status: healthprotocol.StatusServing, wantStatus: health.StatusServing},
		{name: "not serving", code: http.StatusServiceUnavailable, status: healthprotocol.StatusNotServing, wantStatus: health.StatusNotServing},
		{name: "unknown", code: http.StatusServiceUnavailable, status: healthprotocol.StatusUnknown, wantStatus: health.StatusUnknown},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := newHealthServer(t, test.code, `{"status":"`+test.status+`"}`)
			checker := newChecker(t, server.URL)

			status, err := checker.Check(t.Context())
			if err != nil {
				t.Fatalf("Check() error = %v", err)
			}
			if status != test.wantStatus {
				t.Fatalf("Check() status = %v, want %v", status, test.wantStatus)
			}
		})
	}
}

func TestCheckerRejectsInvalidHTTPHealthResponses(t *testing.T) {
	tests := []struct {
		name   string
		code   int
		status string
	}{
		{name: "serving with unavailable", code: http.StatusServiceUnavailable, status: healthprotocol.StatusServing},
		{name: "not serving with success", code: http.StatusOK, status: healthprotocol.StatusNotServing},
		{name: "unknown with success", code: http.StatusOK, status: healthprotocol.StatusUnknown},
		{name: "unexpected HTTP status", code: http.StatusInternalServerError, status: healthprotocol.StatusUnknown},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := newHealthServer(t, test.code, `{"status":"`+test.status+`"}`)
			checker := newChecker(t, server.URL)

			status, err := checker.Check(t.Context())
			if err == nil {
				t.Fatal("Check() error = nil, want error")
			}
			if status != health.StatusUnknown {
				t.Fatalf("Check() status = %v, want %v", status, health.StatusUnknown)
			}
		})
	}
}

func TestCheckerRejectsMalformedAndOversizedResponses(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "malformed JSON", body: `{`},
		{name: "unknown status", body: `{"status":"BROKEN"}`},
		{name: "oversized body", body: strings.Repeat("x", 4<<10+1)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := newHealthServer(t, http.StatusOK, test.body)
			checker := newChecker(t, server.URL)

			status, err := checker.Check(t.Context())
			if err == nil {
				t.Fatal("Check() error = nil, want error")
			}
			if status != health.StatusUnknown {
				t.Fatalf("Check() status = %v, want %v", status, health.StatusUnknown)
			}
		})
	}
}

func TestCheckerHonorsContextDeadline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		<-req.Context().Done()
	}))
	t.Cleanup(server.Close)

	checker := newChecker(t, server.URL)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer cancel()

	status, err := checker.Check(ctx)
	if err == nil {
		t.Fatal("Check() error = nil, want error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Check() error = %v, want context deadline exceeded", err)
	}
	if status != health.StatusUnknown {
		t.Fatalf("Check() status = %v, want %v", status, health.StatusUnknown)
	}
}

func newHealthServer(t *testing.T, code int, body string) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			t.Errorf("request method = %q, want %q", req.Method, http.MethodGet)
		}

		res.WriteHeader(code)
		_, _ = res.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	return server
}

func newChecker(t *testing.T, rawURL string) *healthhttp.Checker {
	t.Helper()

	healthURL, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse health URL: %v", err)
	}

	checker, err := healthhttp.NewChecker(healthURL, &http.Client{Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewChecker() error = %v", err)
	}
	return checker
}
