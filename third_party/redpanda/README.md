# Redpanda Protobuf Inputs

This directory contains dependency-only protobuf schemas maintained by
Redpanda Data. They are not Canterbury APIs and must not produce Canterbury
generated code.

`mcp/v1/annotations.proto` is vendored from
`github.com/redpanda-data/protoc-gen-go-mcp` at the version pinned in `go.mod`.
It defines the `(mcp.v1.tool_name)` method option used by Canterbury's vault
service contract. The upstream project distributes this schema under the
Apache License 2.0.

The directory is a separate local Buf module so schemas under `api/` can import
it without placing third-party packages alongside Canterbury-owned APIs. See
[Dependency Maintenance](../../docs/maintenance.md) before updating it.
