# Canterbury Architecture

Canterbury is planned as a controlled service layer between AI agents or other
integrations and an Obsidian vault. The sync worker and an initial local
vault service with `ReadNote` and `SearchNotes` RPCs are currently implemented.
The Go package structure described here is still the intended shape for service
components as they are added.

## Package Boundaries

The Go service should use a domain-driven structure:

```text
cmd/
  vault-service/
    main.go

internal/
  domain/
    audit/
      event.go
      recorder.go
      errors.go

    vault/
      access.go
      errors.go
      note.go
      repository.go
      search.go

  app/
    auditctx/
      context.go

    auditlog/
      service.go
      record_event.go
      id_generator.go

    auth/
      principal.go

    clock/
      clock.go
      system_clock.go

    idgen/
      ulid.go
      errors.go

    vault/
      service.go
      read_note.go
      search_notes.go
      audit.go
      audit_read.go
      audit_search.go
      errors.go
      note.go

  adapters/
    auditfs/
      recorder.go
      jsonl.go
      record.go
      event_data.go
      file.go
      errors.go

    vaultfs/
      repository.go
      frontmatter.go
      path.go
      read.go
      search.go
      pagination.go
      snippet.go
      sort.go
      match.go
      errors.go

  interfaces/
    connectrpc/
      vault_service.go
      read_note.go
      search_notes.go
      audit_context.go
      correlation.go
      error.go
      log.go
      proto_note.go
      proto_search.go
      proto_struct.go

    mcp/
      tools.go
      read_note.go
      search_notes.go

    rest/
      routes.go
      read_note.go
      search_notes.go
```

Not every directory exists yet. Create packages only when there is implemented
behavior to put in them. The current external interface is Connect/gRPC;
dedicated MCP and REST interfaces remain planned.

## Dependency Direction

Keep dependencies flowing inward:

```text
interfaces -> app -> domain
adapters   -> domain
cmd        -> app + adapters + interfaces
```

- `domain` defines core vocabulary, invariants, errors, and ports. Currently
  includes `domain/vault` and `domain/audit`.
- `app` implements use cases and coordinates policy, audit, and repositories.
  Currently includes `app/vault` for read and search use cases, plus supporting
  packages `app/auditctx`, `app/auditlog`, `app/auth`, `app/clock`, and
  `app/idgen`.
- `adapters` implements ports using external systems such as the vault
  filesystem mirror or an audit store. Currently includes `adapters/vaultfs`
  and `adapters/auditfs`.
- `interfaces` adapts external protocols such as Connect/gRPC, MCP, or REST to
  application use cases. Currently includes `interfaces/connectrpc`; dedicated
  MCP and REST interfaces remain planned.
- `cmd` wires concrete implementations together for an executable.

Domain packages must not import application, adapter, or interface
packages.

## Vault Domain

`internal/domain/vault` owns the note and search vocabulary:

- note identity through normalized vault-relative Markdown paths;
- note content and metadata models;
- resource access scopes declared by frontmatter;
- search query, filter, result, and pagination models;
- repository ports for reading and searching notes.

The vault domain exposes access metadata, but it does not authorize principals.
Authorization belongs in an application/policy layer.

## Audit Domain

`internal/domain/audit` owns the audit event vocabulary:

- audit event models with actor, action, resource, and outcome fields;
- the `Recorder` port through which application use cases publish audit events.

The audit domain does not implement recording; that belongs in `adapters/auditfs`.

## Filesystem Repository

`internal/adapters/vaultfs` implements the initial filesystem-backed vault
repository against the synced local vault mirror. It currently supports safe
Markdown note reads and direct filesystem search. It should continue to:

- read only Markdown notes through normalized vault-relative paths;
- reject traversal, absolute paths, hidden/system paths, and symlink escapes;
- parse YAML frontmatter and Markdown body content;
- remain replaceable by a future SQLite-backed index implementation.

Direct filesystem search currently uses offset-based pagination over each live,
sorted scan. These cursors are best-effort continuation markers rather than
stable snapshot tokens, so callers may see skipped or repeated results if the
vault changes between page requests. A future indexed repository can replace
this with stronger cursor semantics.

The current Connect `SearchNotes` adapter treats `query.text` as one trimmed
search term and does not parse comma-separated term syntax. The filesystem
repository matches text case-insensitively against note content, applies path
and tag filters, and returns snippets without full note content.

## Application Layer

`internal/app/vault` is the home for read and search use cases. It currently
composes the vault repository with scope-based authorization and records audit
events for read and search attempts.

Supporting packages in `internal/app/`:

- `auditctx` carries the correlation and principal context used by audit event
  recording.
- `auditlog` implements audit event recording through the `domain/audit`
  recorder port.
- `auth` defines the principal model used for authorization.
- `clock` provides a `Clock` abstraction and a system-time implementation used
  for audit timestamps.
- `idgen` provides unique ID generation (ULID) used for audit event IDs.

`internal/adapters/auditfs` holds the filesystem append-only audit log
implementation. Future write operations must not commit successfully without an
independent audit record.

`internal/interfaces/connectrpc` is the current external interface. Future
`internal/interfaces/mcp` and future `internal/interfaces/rest` should expose
protocol adapters only. They should translate requests into application use
cases rather than reading vault files directly.
