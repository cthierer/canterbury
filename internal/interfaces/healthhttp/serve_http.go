package healthhttp

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
)

// ServeHTTP returns the resolved health status for GET and HEAD requests.
func (service *healthServiceHandler) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		res.Header().Set("Allow", "GET, HEAD")
		http.Error(res, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := req.Context()
	health, err := service.getStatus(ctx)
	if err != nil {
		status, message := classifyHTTPError(err)
		logHTTPError(ctx, "getting status", err, status)
		http.Error(res, message, status)
		return
	}

	respJSON, err := json.Marshal(health)
	if err != nil {
		slog.ErrorContext(ctx, "marshaling status", "err", err)
		http.Error(res, "internal server error", http.StatusInternalServerError)
		return
	}

	res.Header().Set("Content-Type", "application/json")
	res.Header().Set("Cache-Control", "no-store")
	res.Header().Set("Content-Length", strconv.Itoa(len(respJSON)))

	switch health.Status {
	case statusServing:
		res.WriteHeader(http.StatusOK)
	default:
		res.WriteHeader(http.StatusServiceUnavailable)
	}

	if req.Method == http.MethodHead {
		return
	}

	_, err = res.Write(respJSON)
	if err != nil {
		logHTTPError(ctx, "writing response", err, httpErrorStatus(err))
		return
	}
}

func httpErrorStatus(err error) int {
	status, _ := classifyHTTPError(err)
	return status
}

func classifyHTTPError(err error) (status int, message string) {
	switch {
	case errors.Is(err, context.Canceled):
		return http.StatusRequestTimeout, "request canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return http.StatusRequestTimeout, "request timeout"
	default:
		return http.StatusInternalServerError, "internal server error"
	}
}

func logHTTPError(ctx context.Context, message string, err error, status int) {
	if status == http.StatusRequestTimeout {
		slog.DebugContext(ctx, message, "err", err, "status", status)
		return
	}

	slog.ErrorContext(ctx, message, "err", err, "status", status)
}
