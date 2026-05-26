# Canterbury Auth V1 Design

## Purpose

Canterbury requires signed bearer-token authentication for vault RPCs before a
caller can read or search vault content. The service validates each bearer JWT,
maps the verified `(issuer, subject)` pair to Canterbury vault scopes from a
local TOML policy file, attaches the resolved principal to request context, and
authorizes vault reads and searches against note-declared `access.scopes`.

Canterbury owns the final scope mapping and access decision. External identity
providers, groups, emails, and upstream headers are identity evidence only unless
they are explicitly represented in the local mapping file.

The default local deployment uses Pomerium Core in front of the vault service.
The development workflow can also use the local `dev-auth` helper. Both flows
present the same shape to Canterbury: `Authorization: Bearer <signed JWT>`.

## Implemented Scope

Auth V1 currently provides:

- Signed bearer JWT validation for unary vault service RPCs.
- JWKS-backed signature verification.
- Issuer, audience, expiration, and subject validation.
- Exact `(issuer, subject)` to Canterbury scope mapping from TOML.
- Startup validation for invalid TOML, invalid scopes, duplicate subjects,
  blank issuers, blank subjects, and empty mapped scope lists.
- Startup logging of mapping subject count and mapping file checksum.
- Request-scoped principal propagation through context.
- Scope-based authorization for `ReadNote` and `SearchNotes`.
- `auth.failed` audit events before rejected authentication requests return.
- Actor and policy data on read/search audit events, including issuer, subject
  hash, mapped scopes, and mapping checksum.
- A local `dev-auth` service that mints EdDSA JWTs and serves a JWKS endpoint
  for local development and smoke tests.
- A local Pomerium/Dex Docker Compose stack for deployed-style gateway testing.

Auth V1 does not yet provide:

- Production identity-provider provisioning or account management.
- Dynamic reload of the scope mapping file.
- `auth.succeeded` audit events.
- Authentication for health or reflection endpoints.
- MCP-facing auth behavior, because the MCP interface is not implemented yet.
- Write-path authorization, because write operations are not implemented yet.

## Request Flow

```text
Client or MCP host
  -> Pomerium or dev-auth token issuer
  -> Authorization: Bearer <signed JWT>
  -> Canterbury Connect/gRPC vault handler
  -> audit context interceptor attaches request metadata
  -> auth context interceptor validates JWT
  -> auth mapper resolves Canterbury scopes
  -> principal is attached to context
  -> vault app authorizes read/search against note access scopes
  -> audit events record actor, policy, and outcome
```

The auth interceptor wraps the vault service handler only. Health and reflection
handlers remain available as operational endpoints.

## Configuration

The vault service requires these auth settings:

```env
VAULT_SERVICE_AUTH_ISSUER=devauth.canterbury.local
VAULT_SERVICE_AUTH_AUDIENCE=canterbury.vault.local
VAULT_SERVICE_AUTH_JWKS_URL=http://127.0.0.1:50052/.well-known/jwks.json
VAULT_SERVICE_AUTH_MAPPING_FILE=./sample-auth/scopes.toml
```

The local Pomerium stack uses the same configuration shape with Pomerium as the
issuer and JWKS provider:

```env
VAULT_SERVICE_AUTH_ISSUER=vault.localhost.pomerium.io
VAULT_SERVICE_AUTH_AUDIENCE=vault.localhost.pomerium.io
VAULT_SERVICE_AUTH_JWKS_URL=https://vault.localhost.pomerium.io:8443/.well-known/pomerium/jwks.json
VAULT_SERVICE_AUTH_MAPPING_FILE=/auth/scopes.toml
```

There is no separate no-auth mode for vault RPCs. The old process-wide
`VAULT_SERVICE_AUTH_SCOPES` model has been replaced by per-request principals
resolved from signed assertions.

## Scope Mapping

`VAULT_SERVICE_AUTH_MAPPING_FILE` points to a TOML file outside the vault. The
current format is:

```toml
[[subjects]]
issuer = "devauth.canterbury.local"
subject = "user_123"
scopes = ["personal-agent"]

[[subjects]]
issuer = "vault.localhost.pomerium.io"
subject = "Cgh1c2VyXzEyMxIFbG9jYWw"
scopes = ["personal-agent"]
```

Mapping rules:

- Entries match by exact `issuer` and exact `subject` after surrounding
  whitespace is trimmed.
- Every mapped subject must grant at least one Canterbury scope.
- Every scope must pass `vault.NewScope`.
- Duplicate `(issuer, subject)` entries are rejected at startup.
- Unknown keys in the TOML file are rejected at startup.
- The mapping file loads once at startup.
- The service computes a SHA-256 checksum over the mapping file bytes.
- Startup logs include only the mapping subject count and checksum, never full
  mapping contents.

Subjects should be stable provider identifiers, not email addresses. Email
addresses can change and can be ambiguous across providers.

## Principal Model

After a token verifies and maps successfully, the auth application builds an
`internal/app/auth.Principal` containing:

- `Issuer`: verified and trimmed token issuer.
- `Subject`: verified and trimmed token subject, used inside process memory for
  policy mapping.
- `SubjectHash`: SHA-256 hash of `issuer + "\x00" + subject`, prefixed with
  `sha256:`, used in audit records.
- `Scopes`: mapped Canterbury vault scopes.
- `MappingChecksum`: checksum of the mapping file used to resolve the
  principal.

Vault application methods read the principal from context. They do not store one
process-wide principal on the service.

## Token Validation

`internal/adapters/authjwt` validates JWTs using a JWKS-backed key function and
an explicit allowed-method list. The vault service currently allows `EdDSA` for
local `dev-auth` tokens and `ES256` for local Pomerium assertions.

Validation checks:

- Token parses as a JWT.
- Signing method is allowed.
- Signature validates against JWKS.
- Expiration is present and not expired.
- Issuer matches `VAULT_SERVICE_AUTH_ISSUER`.
- Audience includes `VAULT_SERVICE_AUTH_AUDIENCE`.
- Subject is present and not blank.

Auth failure reasons are normalized before audit logging so raw verifier errors,
claims, and token fragments are not recorded.

## Authorization Behavior

Missing, malformed, expired, wrong-issuer, wrong-audience, bad-signature, and
missing-subject tokens return Connect `Unauthenticated`.

Valid tokens whose `(issuer, subject)` pair is absent from the mapping file are
also rejected at the auth boundary with `Unauthenticated`. Canterbury treats
these callers as unresolved identities rather than zero-scope principals.

Mapped principals whose scopes do not match a note's declared `access.scopes`
continue into the vault app and receive policy denials such as Connect
`PermissionDenied` and `vault.read.denied` audit events.

Notes without `access.scopes` remain default-deny because no principal scope can
match an undeclared resource scope.

## Audit Integration

Authentication failures record `auth.failed` before returning to the caller.
Reasons include:

- `missing_bearer_token`
- `malformed_authorization`
- `malformed_jwt`
- `invalid_signature`
- `expired_token`
- `wrong_issuer`
- `wrong_audience`
- `missing_subject`
- `principal_resolution_failed`

Failure events use this shape:

```json
{
	"event_type": "auth.failed",
	"actor": {
		"issuer": "devauth.canterbury.local",
		"subject_hash": "sha256:...",
		"scopes": []
	},
	"policy": {
		"mapping_checksum": "sha256:...",
		"matched_scopes": [],
		"decision": "deny"
	},
	"outcome": {
		"status": "failed",
		"code": "unauthenticated",
		"duration_ns": 1200000
	},
	"details": {
		"reason": "principal_resolution_failed"
	}
}
```

When the service cannot safely extract a verified issuer or subject, those actor
fields are left empty. When a token verifies but principal resolution fails, the
event may include the verified issuer and stable subject hash.

If an auth failure audit event cannot be written, the request still cannot
proceed past the auth boundary. The interceptor returns an internal error rather
than allowing access without an audit record.

Read and search audit events include authenticated actor and policy context:

- `actor.issuer`
- `actor.subject_hash`
- `actor.scopes`
- `policy.mapping_checksum`
- `policy.matched_scopes`
- `policy.decision`

The implementation does not log bearer tokens, raw JWTs, full claims, raw
emails, raw subjects, full mapping contents, or raw remote addresses.

## Local Development

The local development path uses the same assertion configuration shape as the
Pomerium stack:

1. Start `dev-auth` with `go run ./cmd/dev-auth serve`.
2. Start `vault-service` with `VAULT_SERVICE_AUTH_JWKS_URL` pointing at the
   dev-auth JWKS endpoint.
3. Mint a token with `MintToken`.
4. Send vault requests with `Authorization: Bearer <token>`.

`dev-auth` generates an in-memory Ed25519 key on startup, mints EdDSA JWTs, and
serves its public key at `/.well-known/jwks.json`. Tokens minted before a
`dev-auth` restart do not verify against the new key.

The opt-in local auth smoke test exercises the full local loop:

```bash
make smoke-auth
```

The local Pomerium smoke test exercises the gateway assertion path:

```bash
make smoke-pomerium
```

## Key Packages

- `internal/app/auth`: principal model, token verification port, scope mapper,
  and authenticator.
- `internal/adapters/authfs`: TOML mapping loader and checksum computation.
- `internal/adapters/authjwt`: JWKS-backed JWT verifier.
- `internal/app/authctx`: context helpers for request principals.
- `internal/interfaces/vaultrpc`: auth and audit context interceptors for vault
  RPCs.
- `cmd/dev-auth`: local development auth service.
- `internal/app/devauth`, `internal/adapters/devauthjwt`, and
  `internal/interfaces/devrpc`: local token minting support.

## Verification

The regular repository quality gate covers the Auth V1 unit and integration
tests that do not require Docker:

```bash
make check
```

Additional opt-in smoke tests cover runnable local flows:

```bash
make smoke-auth
make smoke-pomerium
```

The Pomerium smoke test requires the local Docker Compose stack and generated
local Pomerium files.

## Security Notes

- Do not trust raw `X-User`, `X-Email`, or `X-Scopes` headers as the vault
  service security boundary.
- Keep mapping files outside the vault.
- Keep Obsidian account credentials out of the vault service.
- Use stable provider subjects, not emails, in scope mappings.
- Treat auth mapping files as policy.
- Treat audit logs as sensitive security records.
- Prefer adding new issuers through the same JWT/JWKS validation path rather
  than adding parallel auth modes.

## Open Follow-Ups

- Revisit Pomerium MCP experimental features after Canterbury has an MCP
  interface.
