# Canterbury Audit V1 Implementation Notes

## Goal

Design an audit trail that can answer:

```text
Who was allowed to see or change what, through which interface, under which
policy, and what was the outcome?
```

Audit records must live outside the vault. AI agents and vault content should not
be able to rewrite or erase the independent history of access decisions.

For V1, optimize for a simple append-only log that is easy to inspect, test, and
replace later. Prefer JSON Lines (`.jsonl`) as the first storage format.

## V1 Scope

Audit the current read/search service surface and the auth boundary that protects
it:

- Authentication failures.
- Successful note reads.
- Denied note reads.
- Completed searches.
- Search/auth failures where a request was rejected before returning results.

Do not build a full audit query API yet. Do not implement write-operation audit
gating yet, but keep the schema ready for it.

## JSON Lines Format

Use one JSON object per line:

```text
{"id":"01H...","occurred_at":"2026-05-02T18:25:43.123Z","event_type":"vault.read.allowed",...}
{"id":"01H...","occurred_at":"2026-05-02T18:25:44.901Z","event_type":"vault.search.completed",...}
```

V1 file behavior:

- Append one complete record per line.
- Write audit logs outside the vault, configured with an explicit root directory
  such as `VAULT_SERVICE_AUDIT_ROOT=./audit`.
- Rotate logs by event date and writer ID under the audit root. The filesystem
  recorder writes daily JSONL files below year/month directories, for example
  `./audit/2026/05/2026_05_03_vault-service-a_audit.jsonl`.
- Include a per-process audit writer ID in each filename so multiple service
  instances sharing an audit root write separate files. Deployments may set
  `VAULT_SERVICE_AUDIT_WRITER_ID`; otherwise the service generates one from
  hostname, PID, and a short random suffix.
- Use file permissions that prevent casual reads by other local users.
- Open each daily file in append mode; do not rewrite existing records.
- Treat short or partial writes as audit write failures.
- Flush/sync policy can start conservative and be tuned later. For future writes,
  the audit record must be durably written before committing the write.
- If an audit write fails for read/search V1, return an internal error rather
  than serving data without an audit trail.

Why JSONL:

- Easy to append atomically enough for a single-process PoC.
- Easy to inspect with shell tools.
- Easy to ingest into SQLite, DuckDB, object storage, or log systems later.
- Keeps each event self-contained and schema-versioned.

Limitations to remember:

- JSONL is not tamper-proof by itself.
- Compression, signing, hash chaining, retention, and more advanced rotation
  policies are future work.
- Multi-process writes require stronger coordination or moving to SQLite/logging
  infrastructure.

## Event Envelope

Every event should share this shape:

```json
{
	"id": "01HZY...",
	"schema_version": 1,
	"occurred_at": "2026-05-02T18:25:43.123Z",
	"event_type": "vault.read.allowed",
	"request_id": "req_abc123",
	"trace_id": "trace_abc123",
	"actor": {
		"issuer": "https://auth.example.test",
		"subject": "user_123",
		"subject_hash": "sha256:...",
		"scopes": ["personal-agent"]
	},
	"client": {
		"interface": "connectrpc",
		"user_agent": "...",
		"remote_addr_hash": "sha256:..."
	},
	"policy": {
		"mapping_checksum": "sha256:...",
		"matched_scopes": ["personal-agent"],
		"decision": "allow"
	},
	"outcome": {
		"status": "success",
		"code": "ok",
		"duration_ns": 12000000
	},
	"details": {}
}
```

Field guidance:

- `id`: sortable unique event ID. ULID is a good fit, but UUID is acceptable.
- `schema_version`: required so future records can evolve safely.
- `occurred_at`: UTC RFC 3339 timestamp with milliseconds or better.
- `event_type`: stable dotted name.
- `request_id`: generated once per inbound request and reused for related events.
- `trace_id`: optional for now; keep the field if tracing is added.
- `actor.subject`: stable provider subject, not email.
- `actor.subject_hash`: useful when logs should avoid exposing raw subject.
- `actor.scopes`: Canterbury scopes granted after mapping.
- `client.remote_addr_hash`: hash by default; raw addresses are sensitive.
- `policy.mapping_checksum`: checksum of the TOML mapping file loaded by the
  service.
- `policy.matched_scopes`: scopes that allowed access, when applicable.
- `outcome.code`: service/application code such as `ok`, `permission_denied`,
  `unauthenticated`, `invalid_argument`, `not_found`, `unavailable`, or
  `internal`.
- `outcome.duration_ns`: operation duration as a nanosecond count, matching Go's
  `time.Duration` JSON representation.
- `details`: event-specific payload.

## Event Types

### Authentication

`auth.failed`

Record when a request has no usable identity:

- Missing bearer token.
- Malformed `Authorization` header.
- Invalid signature.
- Expired token.
- Wrong issuer.
- Wrong audience/resource.
- Missing subject.

Do not log the token or full claims.

`auth.succeeded`

Optional in V1. Useful during early development, but noisy in production. If
enabled, record only issuer, subject hash, mapped scope count, and request id.

### Vault Reads

`vault.read.allowed`

Record when note content or metadata is returned to a caller.

Suggested details:

```json
{
	"note_ref": {
		"path": "Projects/Canterbury.md",
		"title": "Canterbury"
	},
	"resource_scopes": ["personal-agent"],
	"content_bytes": 4210,
	"content_hash": "sha256:..."
}
```

`vault.read.denied`

Record when an authenticated caller requests a note but lacks matching scopes.

Suggested details:

```json
{
	"note_ref": {
		"path": "Projects/Private.md"
	},
	"resource_scopes": ["private-agent"],
	"reason": "no_matching_scope"
}
```

Do not expose note existence externally unless the API intentionally does so.
The internal audit log may record existence and policy details.

`vault.read.failed`

Record unexpected failures and invalid note paths.

Suggested details:

```json
{
	"note_ref": {
		"path": "Projects/Bad.md"
	},
	"reason": "invalid_note_path"
}
```

### Vault Search

`vault.search.completed`

Record when search results are returned.

Suggested details:

```json
{
	"query": {
		"text_hash": "sha256:...",
		"has_text": true,
		"include_path_prefixes": ["Projects/"],
		"exclude_path_prefixes": [],
		"all_tags": ["canterbury"],
		"any_tags": [],
		"page_size": 10,
		"page_token_hash": null
	},
	"results": {
		"count": 3,
		"returned_refs": ["Projects/Canterbury.md", "Projects/Agent Research.md"],
		"result_set_hash": "sha256:..."
	}
}
```

Default to hashing search text because queries can reveal sensitive user intent.
For local development, a config flag may allow raw query text, but production
defaults should redact/hash.

Store returned note refs because that is the actual disclosure event.

`vault.search.failed`

Record invalid searches, unauthenticated requests, and unexpected failures that
prevent search results from returning.

Suggested details:

```json
{
	"query": {
		"text_hash": "sha256:...",
		"has_text": true
	},
	"reason": "invalid_search_query"
}
```

### MCP Future Events

Keep these names reserved for the MCP interface:

- `mcp.tool.called`
- `mcp.tool.completed`
- `mcp.tool.failed`
- `mcp.tool.denied`

MCP details should include:

- Tool name.
- MCP client or host identity, if available.
- Underlying vault operation event IDs.
- Duration and result size.
- Whether the tool call returned content, refs, or only metadata.

### Future Write Events

Keep these names reserved for write operations:

- `vault.write.proposed`
- `vault.write.audit_recorded`
- `vault.write.committed`
- `vault.write.failed`
- `vault.write.rejected`

Future write invariant:

```text
No write may commit unless the corresponding audit record has already been
successfully recorded outside the vault.
```

## Recorder Interface

Keep the application code independent from JSONL:

```go
type Recorder interface {
	Record(ctx context.Context, event Event) error
}
```

Suggested package direction:

- `internal/domain/audit`: event vocabulary and recorder port.
- `internal/adapters/auditfs`: JSONL filesystem recorder implementation.
- `internal/app/vault`: emits read audit events through the recorder. Search
  audit events are planned next.
- `cmd/vault-service`: wires the configured recorder.

Use a no-op recorder only in tests that explicitly opt into it. The service
runtime should fail closed if audit configuration is required but unavailable.

## Privacy And Security Defaults

- Never log bearer tokens.
- Never log Obsidian credentials.
- Never log note content.
- Do not log full JWT claims.
- Prefer stable provider subject IDs over emails.
- Hash search text by default.
- Hash remote addresses by default.
- Consider the audit log sensitive; protect it like operational security data.
- Keep audit logs outside the vault and outside any synced Obsidian path.

## V1 Acceptance Criteria

- Every successful `ReadNote` response has a corresponding audit event.
- Every denied `ReadNote` authorization decision has a corresponding audit
  event.
- Every successful `SearchNotes` response has a corresponding audit event with
  result count and returned refs.
- Authentication failures are audited without logging tokens.
- Audit write failure prevents returning note content or search results.
- The JSONL log can be parsed line-by-line by tests.
- Date-rotated JSONL files can be discovered under the configured audit root.
- Multiple service instances sharing an audit root write separate per-writer
  daily JSONL files.
- `make check` passes.

## Open Follow-Ups

- Choose exact event ID format: ULID, UUIDv7, or UUIDv4.
- Decide whether to hash values with a deployment-local salt.
- Decide whether V1 should audit `auth.succeeded` or keep only auth failures.
- Decide when to move from JSONL to SQLite or another append-only store.
