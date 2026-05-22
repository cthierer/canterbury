package keyshttp

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
)

func (service *KeyStoreServiceHandler) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		res.Header().Set("Allow", "GET, HEAD")
		http.Error(res, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := req.Context()
	jwks, err := service.getKeySet(ctx)
	if err != nil {
		status, message := classifyHTTPError(err)
		logHTTPError(ctx, "getting key set", err, status)
		http.Error(res, message, status)
		return
	}

	respJSON, err := json.Marshal(jwks)
	if err != nil {
		slog.ErrorContext(ctx, "marshaling key set", "error", err)
		http.Error(res, "internal server error", http.StatusInternalServerError)
		return
	}

	res.Header().Set("Content-Type", "application/jwk-set+json")
	res.Header().Set("Cache-Control", "no-store")
	res.Header().Set("Content-Length", strconv.Itoa(len(respJSON)))
	res.WriteHeader(http.StatusOK)

	if req.Method == http.MethodHead {
		return
	}

	_, err = res.Write(respJSON)
	if err != nil {
		// Can't do much at this point, just log the error
		status, _ := classifyHTTPError(err)
		logHTTPError(ctx, "writing response", err, status)
		return
	}
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
	if status < http.StatusInternalServerError {
		slog.DebugContext(ctx, message, "error", err, "status", status)
		return
	}

	slog.ErrorContext(ctx, message, "error", err, "status", status)
}
