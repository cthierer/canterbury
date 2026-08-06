package healthhttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cthierer/canterbury/internal/domain/health"
)

func TestNewHealthServiceHandlerValidatesApplication(t *testing.T) {
	_, err := newHealthServiceHandler(nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestServeHTTPReturnsResolvedStatus(t *testing.T) {
	tests := []struct {
		name       string
		status     health.Status
		wantCode   int
		wantStatus statusValue
	}{
		{name: "serving", status: health.StatusServing, wantCode: http.StatusOK, wantStatus: statusServing},
		{name: "not serving", status: health.StatusNotServing, wantCode: http.StatusServiceUnavailable, wantStatus: statusNotServing},
		{name: "unknown", status: health.StatusUnknown, wantCode: http.StatusServiceUnavailable, wantStatus: statusUnknown},
		{name: "invalid", status: health.Status(99), wantCode: http.StatusServiceUnavailable, wantStatus: statusUnknown},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := mustHealthHandler(t, &fakeHealthApplication{result: health.Result{Status: test.status}})
			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != test.wantCode {
				t.Fatalf("status code = %d, want %d", rec.Code, test.wantCode)
			}
			if got := rec.Header().Get("Content-Type"); got != "application/json" {
				t.Fatalf("content type = %q, want %q", got, "application/json")
			}
			if got := rec.Header().Get("Cache-Control"); got != "no-store" {
				t.Fatalf("cache control = %q, want %q", got, "no-store")
			}

			var got status
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("unmarshal response: %v", err)
			}
			if got.Status != test.wantStatus {
				t.Fatalf("response status = %q, want %q", got.Status, test.wantStatus)
			}
		})
	}
}

func TestServeHTTPHeadReturnsHeadersWithoutBody(t *testing.T) {
	handler := mustHealthHandler(t, &fakeHealthApplication{result: health.Result{Status: health.StatusServing}})
	req := httptest.NewRequest(http.MethodHead, "/health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Body.String(); got != "" {
		t.Fatalf("body = %q, want empty", got)
	}
	if got := rec.Header().Get("Content-Length"); got == "" {
		t.Fatal("expected content length header")
	}
}

func TestServeHTTPRejectsUnsupportedMethods(t *testing.T) {
	application := &fakeHealthApplication{result: health.Result{Status: health.StatusServing}}
	handler := mustHealthHandler(t, application)
	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
	if got := rec.Header().Get("Allow"); got != "GET, HEAD" {
		t.Fatalf("allow = %q, want %q", got, "GET, HEAD")
	}
	if application.calls != 0 {
		t.Fatalf("application calls = %d, want 0", application.calls)
	}
}

func TestServeHTTPHidesApplicationErrors(t *testing.T) {
	handler := mustHealthHandler(t, &fakeHealthApplication{err: errors.New("sensitive diagnostic")})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if got := rec.Body.String(); got != "internal server error\n" {
		t.Fatalf("body = %q, want generic error", got)
	}
	if strings.Contains(rec.Body.String(), "sensitive") {
		t.Fatalf("body = %q, want no diagnostic details", rec.Body.String())
	}
}

func TestServeHTTPClassifiesCanceledAndTimedOutRequests(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantMsg string
	}{
		{name: "canceled", err: context.Canceled, wantMsg: "request canceled\n"},
		{name: "deadline exceeded", err: context.DeadlineExceeded, wantMsg: "request timeout\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := mustHealthHandler(t, &fakeHealthApplication{err: test.err})
			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusRequestTimeout {
				t.Fatalf("status code = %d, want %d", rec.Code, http.StatusRequestTimeout)
			}
			if got := rec.Body.String(); got != test.wantMsg {
				t.Fatalf("body = %q, want %q", got, test.wantMsg)
			}
		})
	}
}

type fakeHealthApplication struct {
	result health.Result
	err    error
	calls  int
}

func (application *fakeHealthApplication) Check(context.Context) (health.Result, error) {
	application.calls++
	return application.result, application.err
}

func mustHealthHandler(t *testing.T, application HealthApplication) *healthServiceHandler {
	t.Helper()

	handler, err := newHealthServiceHandler(application)
	if err != nil {
		t.Fatalf("NewHealthServiceHandler() error = %v", err)
	}
	return handler
}
