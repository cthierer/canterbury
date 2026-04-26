# Canterbury Architecture

Canterbury is planned as a controlled service layer between AI agents or other
integrations and an Obsidian vault. The sync worker is currently the only
implemented runtime component. The Go package structure described here is the
intended shape for the next service components as they are added.

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
behavior to put in them.

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
- `interfaces` adapts external protocols such as MCP or REST to application use
  cases.
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

`internal/adapters/vaultfs` will implement the vault repository port against the
synced local vault mirror. It should:

- read only Markdown notes through normalized vault-relative paths;
- reject traversal, absolute paths, hidden/system paths, and symlink escapes;
- parse YAML frontmatter and Markdown body content;
- support direct filesystem search for the first implementation;
- remain replaceable by a future SQLite-backed index implementation.

## Future Layers

`internal/app/vault` should become the home for read and search use cases. This
layer will eventually compose the vault repository with policy and audit
services.

`internal/adapters/audit` should hold append-only audit log implementations.
Future write operations must not commit successfully without an independent
audit record.

`internal/interfaces/mcp` and `internal/interfaces/rest` should expose protocol
adapters only. They should translate requests into application use cases rather
than reading vault files directly.
