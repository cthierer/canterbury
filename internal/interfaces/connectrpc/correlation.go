package connectrpc

import (
	"fmt"
	"net/http"
	"strings"

	"connectrpc.com/connect"
	"github.com/cthierer/canterbury/internal/app/idgen"
	"github.com/cthierer/canterbury/internal/domain/audit"
)

const headerRequestID string = "X-Request-ID"
const headerTraceParent string = "traceparent"

func (interceptor *auditContextInterceptor) requestID(req connect.AnyRequest) (audit.RequestID, error) {
	reqIDVal, set := requestIDFromHeader(req.Header())
	if !set {
		reqID, err := interceptor.generateRequestID()
		if err != nil {
			return "", fmt.Errorf("generate request ID: %w", err)
		}

		return reqID, nil
	}

	reqID, err := audit.NewRequestID(reqIDVal)
	if err != nil {
		return "", fmt.Errorf("wrap provided request ID: %w", err)
	}

	return reqID, nil
}

func (interceptor *auditContextInterceptor) setRequestID(res connect.AnyResponse, reqID audit.RequestID) {
	res.Header().Set(headerRequestID, reqID.String())
}

func (*auditContextInterceptor) traceID(req connect.AnyRequest) (audit.TraceID, error) {
	traceIDVal, set := traceIDFromHeader(req.Header())
	if !set {
		return "", nil
	}

	traceID, err := audit.NewTraceID(traceIDVal)
	if err != nil {
		return "", fmt.Errorf("wrap provided trace ID: %w", err)
	}

	return traceID, nil
}

func (interceptor *auditContextInterceptor) generateRequestID() (audit.RequestID, error) {
	id, err := idgen.NewULID(interceptor.clock.Now())
	if err != nil {
		return "", fmt.Errorf("generate new ulid: %w", err)
	}

	reqID, err := audit.NewRequestID(fmt.Sprintf("req_%s", id))
	if err != nil {
		return "", fmt.Errorf("wrap generated request ID: %w", err)
	}

	return reqID, nil
}

func requestIDFromHeader(header http.Header) (string, bool) {
	headerVal := header.Get(headerRequestID)
	trimmed := strings.TrimSpace(headerVal)
	return trimmed, trimmed != ""
}

func traceIDFromHeader(header http.Header) (string, bool) {
	headerVal := header.Get(headerTraceParent)

	trimmed := strings.TrimSpace(headerVal)
	if len(trimmed) == 0 {
		return "", false
	}

	parts := strings.Split(trimmed, "-")
	if len(parts) != 4 {
		return "", false
	}

	version, traceID, parentID, flags := parts[0], parts[1], parts[2], parts[3]
	if !isHex(version, 2) || !isHex(traceID, 32) || !isHex(parentID, 16) || !isHex(flags, 2) {
		return "", false
	}

	if strings.EqualFold(version, "ff") {
		return "", false
	}

	if allZero(traceID) || allZero(parentID) {
		return "", false
	}

	return strings.ToLower(traceID), true
}

func isHex(value string, wantLen int) bool {
	if len(value) != wantLen {
		return false
	}

	for _, char := range value {
		if !strings.ContainsRune("0123456789abcdefABCDEF", char) {
			return false
		}
	}

	return true
}

func allZero(value string) bool {
	for _, char := range value {
		if char != '0' {
			return false
		}
	}

	return true
}
