package mcphttp

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

const headerAuthorization = "Authorization"

type requestMetadata struct {
	bearerToken string
	requestID   string
	traceParent string
}

type requestMetadataKey struct{}

func requireBearer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		token, err := bearerToken(request.Header)
		if err != nil {
			response.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(response, "unauthorized", http.StatusUnauthorized)
			return
		}

		metadata := requestMetadata{
			bearerToken: token,
			requestID:   strings.TrimSpace(request.Header.Get(headerRequestID)),
			traceParent: strings.TrimSpace(request.Header.Get(headerTraceParent)),
		}
		ctx := context.WithValue(request.Context(), requestMetadataKey{}, metadata)
		next.ServeHTTP(response, request.WithContext(ctx))
	})
}

func bearerToken(header http.Header) (string, error) {
	values := header.Values(headerAuthorization)
	if len(values) != 1 {
		return "", fmt.Errorf("exactly one authorization header is required")
	}

	scheme, token, ok := strings.Cut(strings.TrimSpace(values[0]), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", fmt.Errorf("authorization scheme must be bearer")
	}

	token = strings.TrimSpace(token)
	if token == "" || strings.ContainsAny(token, " \t\r\n") {
		return "", fmt.Errorf("bearer token is malformed")
	}

	return token, nil
}
