# Canterbury Auth V1 Implementation Notes

## Goal

Implement one auth path for local and deployed Canterbury: every request carries
a signed bearer JWT/assertion, Canterbury validates it, maps `(issuer, subject)`
to vault scopes from TOML, and authorizes reads/searches against note-declared
`access.scopes`.

The preferred edge for MVP deployments is Pomerium Core/Open Source. Canterbury
still validates signed assertions itself and never trusts raw identity headers as
the security boundary.

## Target Architecture

Request flow:

```text
Client / MCP host
  -> Pomerium or dev token issuer
  -> Authorization: Bearer <signed JWT>
  -> Canterbury Connect/gRPC handler
  -> auth interceptor validates JWT
  -> auth mapper resolves Canterbury scopes
  -> principal attached to context
  -> vault app authorizes read/search
```

Canterbury owns vault scopes. External IdP scopes, groups, and claims are
identity evidence only unless explicitly mapped.

## Config

Replace process-wide `VAULT_SERVICE_AUTH_SCOPES` with required assertion config:

```env
VAULT_SERVICE_ROOT=./sample-vault
VAULT_SERVICE_ADDR=127.0.0.1:50051
VAULT_SERVICE_AUTH_ISSUER=https://auth.example.test
VAULT_SERVICE_AUTH_AUDIENCE=https://canterbury.example.test
VAULT_SERVICE_AUTH_JWKS_URL=http://127.0.0.1:50080/.well-known/jwks.json
VAULT_SERVICE_AUTH_MAPPING_FILE=./sample-auth/scopes.toml
```

For local development, use the same config shape with a local issuer and JWKS
fixture. Avoid adding separate auth modes.

## Scope Mapping TOML

Suggested first format:

```toml
[[subjects]]
issuer = "https://auth.example.test"
subject = "user_123"
scopes = ["personal-agent"]

[[subjects]]
issuer = "https://auth.example.test"
subject = "group_research"
scopes = ["personal-agent", "public-site"]
```

Rules:

- Match by exact `issuer` and `subject`.
- Unknown subjects fail principal resolution and are rejected at the auth
  boundary.
- Each mapped subject must grant at least one scope.
- Scope values must pass `vault.NewScope`.
- Mapping file lives outside the vault.
- Load on startup only for V1.
- Log the mapping count and a file checksum, not full contents.
- Reject duplicate `(issuer, subject)` entries by default.
- Expose the mapping file checksum to audit events as
  `policy.mapping_checksum`.

## Audit Integration

Auth V1 must complete the audit boundary that the current read/search audit
system reserves for authentication.

Authentication failures produce `auth.failed` events before the request is
rejected. Record failures for:

- Missing bearer token.
- Malformed `Authorization` header.
- Malformed JWT.
- Invalid signature.
- Expired token.
- Wrong issuer.
- Wrong audience/resource.
- Missing subject.
- Mapping load or lookup failures that prevent a principal from being resolved.

Failure events should use:

```json
{
	"event_type": "auth.failed",
	"actor": {
		"issuer": "https://auth.example.test",
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
		"reason": "expired_token"
	}
}
```

Do not log bearer tokens, raw JWTs, full claims, raw emails, or raw remote
addresses. If a subject is present, record only a deployment-stable
`subject_hash`. If issuer or subject cannot be safely parsed, leave the unknown
field empty and record a reason.

`auth.succeeded` is optional for V1. If enabled, keep it low-detail and suitable
for production:

- Issuer.
- Subject hash.
- Mapped scope count or mapped Canterbury scopes.
- Mapping checksum.
- Request ID.
- Outcome code `ok`.

The auth interceptor should attach the resolved principal and audit actor data
to context so vault read/search events contain:

- `actor.issuer`
- `actor.subject_hash`
- `actor.scopes`
- `policy.mapping_checksum`

Subjects that validate successfully but have no TOML mapping are not Canterbury
principals. They should produce `auth.failed` with a principal resolution reason
and return `Unauthenticated`. Mapped principals whose scopes do not match a note's
`access.scopes` continue to the vault app and receive policy denials such as
`vault.read.denied` or `PermissionDenied`.

If an auth failure audit event cannot be written, the request must still be
rejected. Prefer returning an internal error only when doing so does not leak
authentication details. The service must never allow a request to proceed past
the auth boundary because audit recording failed.

## Implementation Slices

1. Principal plumbing:
   - Extend `internal/app/auth.Principal` with `Issuer`, `Subject`,
     `SubjectHash`, `Scopes`, and the mapping checksum used to resolve the
     principal.
   - Add context helpers like `WithPrincipal(ctx, principal)` and
     `PrincipalFromContext(ctx)`.
   - Update vault app methods to read the principal from context instead of
     storing one on `Service`.

2. Mapping:
   - Add an auth mapping loader package under `internal/app/auth` or
     `internal/adapters/authfile`.
   - Parse TOML with a small dependency.
   - Validate entries at startup.
   - Provide a mapper interface, such as
     `ScopesFor(issuer, subject string) []vault.Scope`.
   - Compute a stable checksum over the mapping file bytes and include it in
     mapped principals for audit records.

3. JWT validation:
   - Add a verifier that checks bearer extraction, signature, issuer, audience,
     expiry, and subject.
   - Prefer OIDC/JWKS-compatible validation.
   - Keep external provider specifics out of vault app logic.
   - Return structured failure reasons that the interceptor can translate into
     `auth.failed` audit details without logging sensitive claims.

4. Connect integration:
   - Add a Connect interceptor around `VaultService`.
   - Interceptor authenticates, maps scopes, and attaches the principal to
     context.
   - Missing or invalid tokens return `Unauthenticated`.
   - Valid tokens without a TOML `(issuer, subject)` mapping return
     `Unauthenticated`.
   - Valid tokens with no matching note scope return `PermissionDenied` from
     app policy.
   - Interceptor records `auth.failed` events before returning
     `Unauthenticated`.
   - Interceptor populates request-scoped audit metadata used by downstream
     vault events.

5. Local development:
   - Add tiny fake JWKS/token fixtures or a helper command/script later.
   - Bruno requests should include an `Authorization` header.
   - Sample vault notes remain fake and small.

## Security Notes

Do not trust raw `X-User`, `X-Email`, or `X-Scopes` headers as the service auth
boundary. If Pomerium forwards identity, prefer a signed assertion JWT that
Canterbury validates.

Use stable provider subjects, not email addresses, for mappings. Email can
change and can be ambiguous across providers.

Do not log bearer tokens, full claims, or full mapping contents. Logs may include
issuer, a subject hash, mapped scope count, and authorization decision.
Authentication failure audit details should use stable reason codes, not raw
verifier errors that may contain claims or token fragments.

## Test Plan

- Config: missing issuer, audience, JWKS, or mapping fails startup.
- Mapping: valid mapping, unknown subject, empty scopes, invalid TOML, invalid
  scope, and duplicate subject behavior.
- JWT: missing bearer, malformed header, bad signature, wrong issuer, wrong
  audience, expired token, and valid token.
- App: no principal denied, matching scope allowed, non-matching scope denied.
- Connect: auth failures map to `Unauthenticated`; scope failures map to
  `PermissionDenied`.
- Audit: every auth failure records an `auth.failed` event without token or full
  claim data.
- Audit: read/search events include authenticated actor issuer, subject hash,
  scopes, and mapping checksum.
- Audit: unknown subjects produce `auth.failed` with a principal resolution
  reason rather than zero-scope principals.
- Audit: auth failure audit write errors never allow the request to proceed.
- Full check: run `make check`.

## Reference Implementations To Revisit

- Pomerium Core/Open Source: preferred MVP edge gateway.
- Ory: best long-term composable stack if Canterbury needs to own more of the
  OAuth/OIDC flow directly.
- Authentik: good self-hosted IdP candidate if user and group management with a
  UI becomes the near-term need.

## Open Follow-Ups

- Pick the exact Go JWT/OIDC library.
- Decide whether local dev token minting is a Go test helper, CLI command, or
  documented fixture flow.
- Revisit Pomerium MCP experimental features after Canterbury has an MCP
  interface.
