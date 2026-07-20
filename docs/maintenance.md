# Dependency Maintenance

This guide covers Canterbury's pinned code generators, MCP dependencies, and
vendored protobuf inputs. Update them deliberately and review generated changes
before merging.

## Protobuf Generator Model

`buf.gen.yaml` runs the Go generators locally with `go run`:

| Generator                                                          | Version source                                           |
| ------------------------------------------------------------------ | -------------------------------------------------------- |
| `google.golang.org/protobuf/cmd/protoc-gen-go`                     | `google.golang.org/protobuf` in `go.mod`                 |
| `connectrpc.com/connect/cmd/protoc-gen-connect-go`                 | `connectrpc.com/connect` in `go.mod`                     |
| `github.com/redpanda-data/protoc-gen-go-mcp/cmd/protoc-gen-go-mcp` | `github.com/redpanda-data/protoc-gen-go-mcp` in `go.mod` |

These plugins are locally executed and pinned by `go.mod` and `go.sum`; their
source is not copied into this repository. This avoids floating tags such as
`@latest`, keeps the generator and Redpanda runtime on one module version, and
allows Go's module checksum verification to cover generator downloads.

The Redpanda annotation schema is different: Buf needs the `.proto` source to
resolve `(mcp.v1.tool_name)`, so it is vendored under
`third_party/redpanda/mcp/v1/annotations.proto`. `buf.yaml` exposes that
directory as a dependency-only local module. `buf.gen.yaml` generates from
`api/` only, so the third-party schema cannot create a second Go extension
descriptor.

The MCP plugin's `types` list is also intentional. It limits MCP adapter
generation to reviewed services. Do not add a service merely to make generation
convenient; first review every RPC as a potential agent-facing tool and retain
an explicit runtime tool allowlist.

## Update Procedure

1. Review the upstream release notes and migration guidance for every module
   being updated. Check for Go version changes, transport changes, generated API
   changes, and security advisories.
2. Update only the intended modules, using explicit versions:

   ```bash
   go get google.golang.org/protobuf@<version>
   go get connectrpc.com/connect@<version>
   go get github.com/redpanda-data/protoc-gen-go-mcp@<version>
   go get github.com/modelcontextprotocol/go-sdk@<version>
   go mod tidy
   ```

   Omit modules that are not part of the update. Inspect all transitive version
   and `go` directive changes in `go.mod` before continuing.

3. When updating `protoc-gen-go-mcp`, locate the downloaded module with:

   ```bash
   go mod download -json github.com/redpanda-data/protoc-gen-go-mcp@<version>
   ```

   Compare its `proto/mcp/v1/annotations.proto` with the vendored copy. Update
   the vendored schema when the option definition, field number, package, or
   `go_package` changes, while retaining the license attribution.

4. Regenerate all protobuf output:

   ```bash
   make proto-generate
   ```

5. Review generated output rather than accepting it mechanically. Confirm:
   - Only `VaultService` receives MCP-generated code unless another service was
     intentionally approved.
   - The published tools remain exactly `read_note` and `search_notes`.
   - No `devv1mcp` package or generated `mcp.v1` Go package appears.
   - Generated schemas and error behavior have not changed unexpectedly.

6. Run the regular and deployment verification:

   ```bash
   make check
   make smoke-pomerium
   ```

   The Pomerium smoke test requires the generated local configuration and the
   Docker Compose stack described in [Local Pomerium Stack](local-pomerium.md).

Commit the module files, vendored schema changes, and regenerated output
together so each dependency update remains reproducible.
