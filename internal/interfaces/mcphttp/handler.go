package mcphttp

import (
	"net/http"

	"github.com/cthierer/canterbury/gen/go/canterbury/vault/v1/vaultv1connect"
	"github.com/cthierer/canterbury/gen/go/canterbury/vault/v1/vaultv1mcp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/redpanda-data/protoc-gen-go-mcp/pkg/runtime"
	"github.com/redpanda-data/protoc-gen-go-mcp/pkg/runtime/gosdk"
)

// Path is the HTTP route for the MCP Streamable HTTP transport.
const Path = "/mcp"

// NewHandler creates a stateless MCP HTTP handler backed by the vault client.
func NewHandler(
	vaultClient vaultv1connect.VaultServiceClient,
	serverName string,
	serverVersion string,
) http.Handler {
	rawServer, mcpServer := gosdk.NewServer(serverName, serverVersion)
	vaultv1mcp.ForwardToConnectVaultServiceClient(
		mcpServer,
		vaultClient,
		runtime.WithToolFilter(isExposedTool),
	)

	mcpHandler := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return rawServer },
		&mcp.StreamableHTTPOptions{
			Stateless:    true,
			JSONResponse: true,
		},
	)

	mux := http.NewServeMux()
	mux.Handle(Path, requireBearer(mcpHandler))
	return mux
}

func isExposedTool(name string) bool {
	switch name {
	case vaultv1mcp.VaultService_ReadNoteTool.Name,
		vaultv1mcp.VaultService_SearchNotesTool.Name:
		return true
	default:
		return false
	}
}
