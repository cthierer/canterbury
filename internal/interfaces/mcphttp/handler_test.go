package mcphttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	vaultv1 "github.com/cthierer/canterbury/gen/go/canterbury/vault/v1"
	"github.com/cthierer/canterbury/gen/go/canterbury/vault/v1/vaultv1connect"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const testUserAgent = "canterbury-mcp-server/test"

func TestBearerToken(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		want   string
		ok     bool
	}{
		{name: "valid", values: []string{"Bearer token-value"}, want: "token-value", ok: true},
		{name: "case insensitive", values: []string{"bearer token-value"}, want: "token-value", ok: true},
		{name: "missing"},
		{name: "duplicate", values: []string{"Bearer first", "Bearer second"}},
		{name: "wrong scheme", values: []string{"Basic token"}},
		{name: "empty token", values: []string{"Bearer "}},
		{name: "token whitespace", values: []string{"Bearer token value"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			header := make(http.Header)
			for _, value := range test.values {
				header.Add(headerAuthorization, value)
			}

			got, err := bearerToken(header)
			if test.ok && (err != nil || got != test.want) {
				t.Fatalf("bearerToken() = %q, %v, want %q, nil", got, err, test.want)
			}
			if !test.ok && err == nil {
				t.Fatalf("bearerToken() = %q, nil, want error", got)
			}
		})
	}
}

func TestForwardMetadataInterceptor(t *testing.T) {
	interceptor := NewForwardMetadataInterceptor(testUserAgent)
	request := connect.NewRequest(&vaultv1.ReadNoteRequest{})
	ctx := context.WithValue(context.Background(), requestMetadataKey{}, requestMetadata{
		bearerToken: "test-token",
		requestID:   "request-123",
		traceParent: "00-0123456789abcdef0123456789abcdef-0123456789abcdef-01",
	})

	_, err := interceptor.WrapUnary(func(_ context.Context, got connect.AnyRequest) (connect.AnyResponse, error) {
		if value := got.Header().Get(headerAuthorization); value != "Bearer test-token" {
			t.Fatalf("Authorization = %q", value)
		}
		if value := got.Header().Get(headerRequestID); value != "request-123" {
			t.Fatalf("X-Request-ID = %q", value)
		}
		if value := got.Header().Get(headerTraceParent); value != "00-0123456789abcdef0123456789abcdef-0123456789abcdef-01" {
			t.Fatalf("traceparent = %q", value)
		}
		if value := got.Header().Get("User-Agent"); value != testUserAgent {
			t.Fatalf("User-Agent = %q", value)
		}
		return connect.NewResponse(&vaultv1.ReadNoteResponse{}), nil
	})(ctx, request)
	if err != nil {
		t.Fatalf("interceptor error = %v", err)
	}

	_, err = interceptor.WrapUnary(func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
		t.Fatal("next called without request metadata")
		return nil, nil
	})(context.Background(), connect.NewRequest(&vaultv1.ReadNoteRequest{}))
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("missing metadata code = %v, want unauthenticated", connect.CodeOf(err))
	}
}

func TestHandlerForwardsAuthorizedTools(t *testing.T) {
	vault := &recordingVaultHandler{}
	vaultServer := newVaultServer(t, vault)
	mcpServer := httptest.NewServer(newTestHandler(vaultServer.URL, time.Second))
	defer mcpServer.Close()

	session := connectMCPClient(t, mcpServer.URL+Path, "first-token", "request-123")
	tools, err := session.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	names := make([]string, 0, len(tools.Tools))
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"read_note", "search_notes"}) {
		t.Fatalf("tool names = %v", names)
	}

	readResult, err := session.CallTool(t.Context(), &mcp.CallToolParams{
		Name: "read_note",
		Arguments: map[string]any{
			"ref": map[string]any{"path": "Projects/Canterbury.md"},
		},
	})
	if err != nil || readResult.IsError {
		t.Fatalf("read_note result = %+v, error = %v", readResult, err)
	}
	readJSON, err := json.Marshal(readResult.StructuredContent)
	if err != nil || !strings.Contains(string(readJSON), "Projects/Canterbury.md") {
		t.Fatalf("read_note structured content = %s, error = %v", readJSON, err)
	}

	searchResult, err := session.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "search_notes",
		Arguments: map[string]any{"query": map[string]any{"text": "Canterbury"}},
	})
	if err != nil || searchResult.IsError {
		t.Fatalf("search_notes result = %+v, error = %v", searchResult, err)
	}

	requests := vault.snapshot()
	if len(requests) != 2 {
		t.Fatalf("vault request count = %d, want 2", len(requests))
	}
	for _, request := range requests {
		if request.authorization != "Bearer first-token" || request.requestID != "request-123" {
			t.Fatalf("forwarded metadata = %+v", request)
		}
		if request.traceParent == "" || request.userAgent != testUserAgent {
			t.Fatalf("forwarded trace and user agent = %+v", request)
		}
	}

	secondSession := connectMCPClient(t, mcpServer.URL+Path, "second-token", "request-456")
	_, err = secondSession.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "read_note",
		Arguments: map[string]any{"ref": map[string]any{"path": "Public/Service Brief.md"}},
	})
	if err != nil {
		t.Fatalf("second read_note error = %v", err)
	}
	last := vault.snapshot()[2]
	if last.authorization != "Bearer second-token" || last.requestID != "request-456" {
		t.Fatalf("second request metadata = %+v", last)
	}
}

func TestHandlerRequiresBearer(t *testing.T) {
	server := httptest.NewServer(newTestHandler("http://127.0.0.1:1", time.Second))
	defer server.Close()

	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL+Path, strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("NewRequestWithContext() error = %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	t.Cleanup(func() {
		if err := response.Body.Close(); err != nil {
			t.Errorf("close response body: %v", err)
		}
	})
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.StatusCode)
	}
	if got := response.Header.Get("WWW-Authenticate"); got != "Bearer" {
		t.Fatalf("WWW-Authenticate = %q", got)
	}
}

func TestHandlerReturnsDownstreamErrors(t *testing.T) {
	tests := []struct {
		name string
		code connect.Code
	}{
		{name: "permission denied", code: connect.CodePermissionDenied},
		{name: "unavailable", code: connect.CodeUnavailable},
		{name: "deadline exceeded", code: connect.CodeDeadlineExceeded},
		{name: "canceled", code: connect.CodeCanceled},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			vault := &recordingVaultHandler{
				readErr: connect.NewError(test.code, errors.New(test.name)),
			}
			vaultServer := newVaultServer(t, vault)
			mcpServer := httptest.NewServer(newTestHandler(vaultServer.URL, time.Second))
			defer mcpServer.Close()
			session := connectMCPClient(t, mcpServer.URL+Path, "token", "")

			result, err := session.CallTool(t.Context(), &mcp.CallToolParams{
				Name:      "read_note",
				Arguments: map[string]any{"ref": map[string]any{"path": "Private.md"}},
			})
			if err != nil {
				t.Fatalf("CallTool() transport error = %v", err)
			}
			if !result.IsError {
				t.Fatalf("CallTool() result = %+v, want tool error", result)
			}
		})
	}
}

func TestHandlerAppliesVaultRequestTimeout(t *testing.T) {
	vault := &recordingVaultHandler{readDelay: 250 * time.Millisecond}
	vaultServer := newVaultServer(t, vault)
	mcpServer := httptest.NewServer(newTestHandler(vaultServer.URL, 10*time.Millisecond))
	defer mcpServer.Close()
	session := connectMCPClient(t, mcpServer.URL+Path, "token", "")

	result, err := session.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "read_note",
		Arguments: map[string]any{"ref": map[string]any{"path": "Slow.md"}},
	})
	if err != nil {
		t.Fatalf("CallTool() transport error = %v", err)
	}
	if !result.IsError {
		t.Fatalf("CallTool() result = %+v, want timeout tool error", result)
	}
}

type recordingVaultHandler struct {
	mu        sync.Mutex
	requests  []recordedRequest
	readErr   error
	readDelay time.Duration
}

type recordedRequest struct {
	authorization string
	requestID     string
	traceParent   string
	userAgent     string
}

func (handler *recordingVaultHandler) ReadNote(
	ctx context.Context,
	request *connect.Request[vaultv1.ReadNoteRequest],
) (*connect.Response[vaultv1.ReadNoteResponse], error) {
	handler.record(request.Header())
	if handler.readDelay > 0 {
		select {
		case <-time.After(handler.readDelay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if handler.readErr != nil {
		return nil, handler.readErr
	}
	return connect.NewResponse(&vaultv1.ReadNoteResponse{Note: &vaultv1.Note{
		Ref:     request.Msg.Ref,
		Content: "test content",
	}}), nil
}

func (handler *recordingVaultHandler) SearchNotes(
	_ context.Context,
	request *connect.Request[vaultv1.SearchNotesRequest],
) (*connect.Response[vaultv1.SearchNotesResponse], error) {
	handler.record(request.Header())
	return connect.NewResponse(&vaultv1.SearchNotesResponse{Results: []*vaultv1.SearchNoteResult{{
		Ref:     &vaultv1.NoteRef{Path: "Projects/Canterbury.md"},
		Snippet: "Canterbury",
	}}}), nil
}

func (handler *recordingVaultHandler) record(header http.Header) {
	handler.mu.Lock()
	defer handler.mu.Unlock()
	handler.requests = append(handler.requests, recordedRequest{
		authorization: header.Get(headerAuthorization),
		requestID:     header.Get(headerRequestID),
		traceParent:   header.Get(headerTraceParent),
		userAgent:     header.Get("User-Agent"),
	})
}

func (handler *recordingVaultHandler) snapshot() []recordedRequest {
	handler.mu.Lock()
	defer handler.mu.Unlock()
	return slices.Clone(handler.requests)
}

func newVaultServer(t *testing.T, vault vaultv1connect.VaultServiceHandler) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	path, handler := vaultv1connect.NewVaultServiceHandler(vault)
	mux.Handle(path, handler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func newTestHandler(baseURL string, timeout time.Duration) http.Handler {
	httpClient := &http.Client{Timeout: timeout}
	vaultClient := vaultv1connect.NewVaultServiceClient(
		httpClient,
		baseURL,
		connect.WithInterceptors(NewForwardMetadataInterceptor(testUserAgent)),
	)
	return NewHandler(vaultClient, "canterbury-test", "test")
}

func connectMCPClient(t *testing.T, endpoint, token, requestID string) *mcp.ClientSession {
	t.Helper()
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		clone := request.Clone(request.Context())
		clone.Header = request.Header.Clone()
		clone.Header.Set(headerAuthorization, "Bearer "+token)
		if requestID != "" {
			clone.Header.Set(headerRequestID, requestID)
		}
		clone.Header.Set(headerTraceParent, "00-0123456789abcdef0123456789abcdef-0123456789abcdef-01")
		return http.DefaultTransport.RoundTrip(clone)
	})}
	client := mcp.NewClient(&mcp.Implementation{Name: "canterbury-test", Version: "test"}, nil)
	session, err := client.Connect(t.Context(), &mcp.StreamableClientTransport{
		Endpoint:             endpoint,
		HTTPClient:           httpClient,
		DisableStandaloneSSE: true,
		MaxRetries:           -1,
	}, nil)
	if err != nil {
		t.Fatalf("connect MCP client: %v", err)
	}
	t.Cleanup(func() {
		_ = session.Close()
	})
	return session
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
