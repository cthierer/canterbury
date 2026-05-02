---
access:
  scopes:
    - personal-agent
tags:
  - project
  - canterbury
  - ai
status: active
owner: local-demo
---

# Canterbury

Canterbury is a controlled service layer for experimenting with AI access to an
Obsidian vault.

This note is intentionally scoped to `personal-agent`, so it should be readable
when the local vault service starts with:

```env
VAULT_SERVICE_AUTH_SCOPES=personal-agent
```

## Demo Checks

- Search for `Canterbury` to find this project note.
- Filter by the `project` or `canterbury` tags to exercise metadata search.
- Read `Projects/Canterbury.md` to confirm frontmatter parsing and content
  stripping.

## Current Focus

The implemented service reads Markdown files from a filesystem vault mirror,
parses frontmatter, and applies note-declared access scopes before returning
results.
