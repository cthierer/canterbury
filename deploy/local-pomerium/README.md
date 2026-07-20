# Local Pomerium Setup

This directory contains development-only templates for running Canterbury behind
Pomerium with Docker Compose.

Run `scripts/setup-local-pomerium.mts` from the repository root to generate the
local-only config, private keys, TLS certificate, and shared secrets. Generated
files are written to `.generated/` and `local.env`, which are ignored by Git.
