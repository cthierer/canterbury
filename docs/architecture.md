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
    vault/
      access.go
      errors.go
      note.go
      repository.go
      search.go

  app/
    vault/
      service.go
      read_note.go
      search_notes.go

  adapters/
    vaultfs/
      repository.go
      frontmatter.go
      path.go
      search.go

    audit/
      log.go

  interfaces/
    connectrpc/
      vault_service.go
      read_note.go
      search_notes.go

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

- `domain` defines core vocabulary, invariants, errors, and ports.
- `app` implements use cases and coordinates policy, audit, and repositories.
- `adapters` implements ports using external systems such as the vault
  filesystem mirror or an audit store.
- `interfaces` adapts external protocols such as Connect/gRPC, MCP, or REST to
  application use cases.
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

## Future Layers

`internal/app/vault` is the home for read and search use cases. It currently
composes the vault repository with scope-based authorization and records audit
events for read attempts.

`internal/adapters/auditfs` holds the filesystem append-only audit log
implementation. Future write operations must not commit successfully without an
independent audit record.

`internal/interfaces/connectrpc`, future `internal/interfaces/mcp`, and future
`internal/interfaces/rest` should expose protocol adapters only. They should
translate requests into application use cases rather than reading vault files
directly.
