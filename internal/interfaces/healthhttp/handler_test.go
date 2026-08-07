package healthhttp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cthierer/canterbury/internal/domain/health"
)

func TestNewHandlerValidatesApplications(t *testing.T) {
	application := &fakeHealthApplication{result: health.Result{Status: health.StatusServing}}

	t.Run("requires live application", func(t *testing.T) {
		_, err := NewHandler(nil, application)
		if err == nil || !strings.Contains(err.Error(), "live check") {
			t.Fatalf("NewHandler() error = %v, want live check error", err)
		}
	})

	t.Run("requires ready application", func(t *testing.T) {
		_, err := NewHandler(application, nil)
		if err == nil || !strings.Contains(err.Error(), "ready check") {
			t.Fatalf("NewHandler() error = %v, want ready check error", err)
		}
	})
}

func TestHandlerRoutesLiveAndReadyChecksIndependently(t *testing.T) {
	liveApplication := &fakeHealthApplication{result: health.Result{Status: health.StatusServing}}
	readyApplication := &fakeHealthApplication{result: health.Result{Status: health.StatusNotServing}}
	handler, err := NewHandler(liveApplication, readyApplication)
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}

	liveResponse := serveHealthGET(handler, livePath)
	if liveResponse.Code != http.StatusOK {
		t.Fatalf("live status code = %d, want %d", liveResponse.Code, http.StatusOK)
	}
	if liveApplication.calls != 1 || readyApplication.calls != 0 {
		t.Fatalf("calls after live check = live %d, ready %d; want live 1, ready 0", liveApplication.calls, readyApplication.calls)
	}

	readyResponse := serveHealthGET(handler, readyPath)
	if readyResponse.Code != http.StatusServiceUnavailable {
		t.Fatalf("ready status code = %d, want %d", readyResponse.Code, http.StatusServiceUnavailable)
	}
	if liveApplication.calls != 1 || readyApplication.calls != 1 {
		t.Fatalf("calls after ready check = live %d, ready %d; want live 1, ready 1", liveApplication.calls, readyApplication.calls)
	}

	missingResponse := serveHealthGET(handler, "/missing")
	if missingResponse.Code != http.StatusNotFound {
		t.Fatalf("missing status code = %d, want %d", missingResponse.Code, http.StatusNotFound)
	}
}

func TestHandlerMountsUnderHealthPrefix(t *testing.T) {
	liveApplication := &fakeHealthApplication{result: health.Result{Status: health.StatusServing}}
	readyApplication := &fakeHealthApplication{result: health.Result{Status: health.StatusServing}}
	healthHandler, err := NewHandler(liveApplication, readyApplication)
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/health/", http.StripPrefix("/health", healthHandler))

	for _, path := range []string{"/health/live", "/health/ready"} {
		response := serveHealthGET(mux, path)
		if response.Code != http.StatusOK {
			t.Errorf("GET %s status code = %d, want %d", path, response.Code, http.StatusOK)
		}
	}

	if liveApplication.calls != 1 || readyApplication.calls != 1 {
		t.Fatalf("mounted handler calls = live %d, ready %d; want 1 each", liveApplication.calls, readyApplication.calls)
	}
}

func serveHealthGET(handler http.Handler, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}
