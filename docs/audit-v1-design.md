# Canterbury Audit System Design

## Purpose

Canterbury records an independent audit trail for controlled vault access. The
audit system is designed to answer:

```text
Who was allowed to see or change what, through which interface, under which
policy, and what was the outcome?
```

Audit records live outside the vault. Vault content and AI agents that access the
vault through Canterbury should not be able to rewrite or erase the history of
access decisions.

The current implementation covers the local vault service read and search paths
and authentication failure events. Authentication design details live in
`docs/auth-v1-design.md`.

## Implemented Scope

The current audit system records:

- Successful note reads.
- Denied note reads.
- Note read failures, such as invalid paths, missing notes, unavailable vaults,
  and unexpected repository errors.
- Completed searches.
- Search failures, such as invalid queries, unavailable vaults, and unexpected
  repository errors.
- Authentication failures.

The current audit system does not yet provide:

- Authentication success events.
- A query API for audit records.
- Write-operation audit gating.
- Tamper-proof storage, signing, hash chaining, compression, or retention
  policies.

## Architecture

The audit implementation is split across domain, application, adapter, and
interface layers:

- `internal/domain/audit` defines the event vocabulary and recorder port.
- `internal/app/auditlog` completes event envelopes with timestamps and ULID
  event IDs, then delegates persistence to a recorder.
- `internal/app/vault` emits read and search audit events through an
  `AuditLogger` interface.
- `internal/adapters/auditfs` persists audit events as JSON Lines files on the
  local filesystem.
- `internal/interfaces/vaultrpc` attaches request-scoped audit metadata, such
  as request IDs, trace IDs, user agents, and hashed remote addresses.
- `cmd/vault-service` wires the configured filesystem recorder into the vault
  service runtime.

Application code depends on the recorder port rather than JSONL storage details:

```go
type Recorder interface {
	Record(ctx context.Context, event Event) error
}
```

The vault service requires an audit logger at startup. Tests may use fake
recorders, but production runtime should not silently fall back to no-op audit
logging.

## Configuration

The vault service requires an explicit audit root:

```env
VAULT_SERVICE_AUDIT_ROOT=./audit
```

The audit root should be outside the vault and outside any synced Obsidian path.
The service creates the root directory if needed and resolves it to an absolute
path before writing records.

Deployments may set a stable writer ID:

```env
VAULT_SERVICE_AUDIT_WRITER_ID=vault-service-a
```

If `VAULT_SERVICE_AUDIT_WRITER_ID` is unset, the filesystem recorder generates a
writer ID from hostname, process ID, and a short random suffix.

The Connect audit metadata interceptor also requires:

```env
VAULT_SERVICE_AUDIT_HMAC_KEY=<base64-encoded 32+ byte key>
```

This key is used to write keyed HMAC-SHA256 hashes of remote client addresses
instead of raw addresses.

## JSON Lines Storage

The filesystem recorder writes one JSON object per line:

```text
{"id":"01K...","schema_version":1,"occurred_at":"2026-05-04T01:51:05.593009117Z","event_type":"vault.read.allowed",...}
{"id":"01K...","schema_version":1,"occurred_at":"2026-05-04T01:51:08.018283102Z","event_type":"vault.search.completed",...}
```

File behavior:

- Append one complete record per line.
- Rotate logs by event date and writer ID under the audit root.
- Write daily JSONL files below year/month directories, for example
  `./audit/2026/05/2026_05_04_vault-service-a_audit.jsonl`.
- Open each daily file with `O_APPEND`; existing records are not rewritten.
- Create directories with `0700` permissions.
- Create audit files with `0600` permissions.
- Treat short or partial writes as audit write failures.

JSONL was chosen because it is easy to inspect with shell tools, straightforward
to test line by line, and simple to ingest into SQLite, DuckDB, object storage,
or logging infrastructure later. It is not tamper-proof by itself.

The per-writer filename prevents multiple service instances from sharing the
same daily log file. Multiple writers may share an audit root, but each writer
should use a distinct writer ID.

## Event Envelope

Every audit event uses the same top-level envelope:

```json
{
	"id": "01KQRAX4BSET20408J01CMEXME",
	"schema_version": 1,
	"occurred_at": "2026-05-04T01:51:05.593009117Z",
	"event_type": "vault.read.allowed",
	"request_id": "req_01KQRAX4BR6JBCNJCZ6PM5VEQX",
	"trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
	"actor": {
		"issuer": "self",
		"subject_hash": "",
		"scopes": []
	},
	"client": {
		"interface": "connectrpc",
		"user_agent": "grpc-node-js/1.13.4",
		"remote_addr_hash": "hmac-sha256:..."
	},
	"policy": {
		"mapping_checksum": "",
		"matched_scopes": ["personal-agent"],
		"decision": "allow"
	},
	"outcome": {
		"status": "success",
		"code": "ok",
		"duration_ns": 3814038
	},
	"details": {}
}
```

Field guidance:

- `id`: sortable unique event ID generated as a ULID.
- `schema_version`: event schema version. The current version is `1`.
- `occurred_at`: UTC RFC 3339 timestamp.
- `event_type`: stable dotted event name.
- `request_id`: generated once per inbound request, or accepted from
  `X-Request-ID` when valid.
- `trace_id`: W3C `traceparent` trace ID when supplied.
- `actor.issuer`: identity issuer for the authenticated principal when known.
- `actor.subject_hash`: hashed stable subject for the authenticated principal
  when known.
- `actor.scopes`: Canterbury scopes granted to the authenticated principal.
- `client.interface`: interface that received the request, such as
  `connectrpc` or `service`.
- `client.user_agent`: caller user agent when available.
- `client.remote_addr_hash`: keyed hash of the remote address when available.
- `policy.mapping_checksum`: checksum of the auth mapping file used to resolve
  the principal.
- `policy.matched_scopes`: note scopes that matched the caller principal.
- `policy.decision`: `allow` or `deny`.
- `outcome.status`: `success`, `failed`, or `error`.
- `outcome.code`: stable result code, such as `ok`, `permission_denied`,
  `invalid_argument`, `not_found`, `unavailable`, or `internal`.
- `outcome.duration_ns`: operation duration as nanoseconds.
- `details`: event-specific payload.

Auth failure events omit actor fields when the service cannot safely extract a
verified issuer or subject. When a signed token verifies but principal
resolution fails, the event may include the issuer and a subject hash while
still omitting raw subject values and bearer token data.

## Request Metadata

The Connect interface attaches audit metadata to unary vault RPCs:

- Accepts a caller-provided `X-Request-ID` when valid.
- Generates a `req_<ULID>` request ID when the header is absent.
- Returns the request ID on successful and error responses.
- Extracts W3C `traceparent` trace IDs when present and valid.
- Records the protocol interface as `connectrpc`.
- Records a trimmed user agent.
- Records an HMAC-SHA256 hash of the remote client address using
  `VAULT_SERVICE_AUDIT_HMAC_KEY`.

Invalid request IDs are rejected before the vault application runs.

## Event Types

### `vault.read.allowed`

Recorded when note content or metadata is returned to a caller.

Details:

```json
{
	"note_ref": {
		"path": "Projects/Canterbury.md",
		"title": "Canterbury"
	},
	"resource_scopes": ["personal-agent"],
	"content_bytes": 703
}
```

The event includes byte length, but does not include note content.

### `vault.read.denied`

Recorded when the vault service finds a note but the configured principal lacks
a matching scope.

Details:

```json
{
	"note_ref": {
		"path": "Private/Unscoped Draft.md",
		"title": ""
	},
	"resource_scopes": [],
	"reason": "no_matching_scope"
}
```

The service returns a permission-denied error. The internal audit event may
include note existence and policy details even when the external API should not
reveal more than the current contract allows.

### `vault.read.failed`

Recorded when a read fails before content can be returned.

Details:

```json
{
	"note_ref": {
		"path": "Projects/Missing.md",
		"title": ""
	},
	"reason": "note_not_found"
}
```

Read failure reasons include:

- `invalid_note_path`
- `note_not_found`
- `vault_unavailable`
- `repository_error`

### `vault.search.completed`

Recorded when search results are returned.

Details:

```json
{
	"query": {
		"text_hash": "sha256:...",
		"has_text": true,
		"include_path_prefixes": ["Projects"],
		"exclude_path_prefixes": [],
		"all_tags": ["project"],
		"any_tags": [],
		"page_size": 25,
		"page_token_hash": null
	},
	"results": {
		"count": 2,
		"returned_refs": ["Projects/Canterbury.md", "Projects/Agent Research.md"],
		"result_set_hash": "sha256:..."
	}
}
```

Search text and page tokens are hashed by default because queries can reveal
sensitive user intent. Returned refs are recorded because they represent the
actual disclosure event.

### `vault.search.failed`

Recorded when a search fails before results can be returned.

Details:

```json
{
	"query": {
		"text_hash": "sha256:...",
		"has_text": true,
		"include_path_prefixes": [],
		"exclude_path_prefixes": [],
		"all_tags": [],
		"any_tags": [],
		"page_size": 25,
		"page_token_hash": null
	},
	"reason": "invalid_search_query"
}
```

Search failure reasons include:

- `invalid_search_query`
- `vault_unavailable`
- `repository_error`

## Failure Behavior

For read and search operations, audit logging is part of the operation contract.
If Canterbury cannot write the required audit event, the service returns an
internal error instead of returning note content or search results.

This fail-closed behavior is intentionally conservative. Future write operations
must go further: no vault write may commit unless the corresponding independent
audit record has already been durably recorded outside the vault.

## Privacy And Security Defaults

- Never log Obsidian credentials.
- Never log bearer tokens.
- Never log full JWT claims.
- Never log note content.
- Hash search text by default.
- Hash page tokens by default.
- Hash remote addresses with a deployment-local HMAC key.
- Keep audit logs outside the vault and outside synced Obsidian paths.
- Treat audit logs as sensitive operational security data.

## Reserved Future Events

Authentication event names are reserved by Auth V1:

- `auth.failed`
- `auth.succeeded`

Dedicated MCP-layer events are reserved for future audit expansion. Current MCP
tool calls produce the mandatory underlying vault read/search records only:

- `mcp.tool.called`
- `mcp.tool.completed`
- `mcp.tool.failed`
- `mcp.tool.denied`

Write events are reserved for future write operations:

- `vault.write.proposed`
- `vault.write.audit_recorded`
- `vault.write.committed`
- `vault.write.failed`
- `vault.write.rejected`

## Operational Limitations

JSONL files are a pragmatic V1 storage format, not a complete audit security
system. Known limitations:

- Records are not cryptographically signed.
- Records are not hash-chained.
- Retention and compression are not implemented.
- There is no audit query API.
- Multi-process deployments should keep distinct writer IDs, or move to a
  stronger shared logging backend later.

Future storage backends may replace JSONL with SQLite, object storage, or a
dedicated log system without changing application use cases, as long as they
preserve the recorder contract and fail-closed behavior.
