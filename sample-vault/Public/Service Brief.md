---
access:
  scopes:
    - personal-agent
    - public-site
tags:
  - public
  - canterbury
status: ready
audience: demo
---

# Service Brief

Canterbury is early software for reading scoped Markdown notes through a
controlled local service.

This note declares both `personal-agent` and `public-site` so it can be used to
test multiple caller scope configurations without changing the file content.

## Highlights

- Access is default deny.
- Notes opt in with `access.scopes`.
- Current reads and searches are local development paths.
- Audit logging and MCP tools are planned but not implemented yet.
