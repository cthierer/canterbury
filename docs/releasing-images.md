# Publishing Canterbury Images

Canterbury publishes deployment images to GitHub Container Registry (GHCR) from
this repository. This document covers the release interface and the manual
actions that remain outside Git.

## Packages And Compatibility

The canonical Bake graph builds these packages for `linux/amd64` only:

| Component     | Package                                     |
| ------------- | ------------------------------------------- |
| MCP server    | `ghcr.io/cthierer/canterbury-mcp-server`    |
| Vault service | `ghcr.io/cthierer/canterbury-vault-service` |
| Sync worker   | `ghcr.io/cthierer/canterbury-sync`          |

Every image carries the OCI source label
`org.opencontainers.image.source=https://github.com/cthierer/canterbury`, which
links its package to this repository. The Bake graph also attaches the exact
source revision, version, commit-created time, MIT license, title, and
description.

## Immutable Tags And Digests

Every successful `main` publish creates exactly one SHA tag per package:

```text
sha-<40-character-commit-sha>
```

If the checked-out commit has exactly one Git tag matching `vMAJOR.MINOR.PATCH`
or a Docker-compatible SemVer prerelease such as `v1.2.3-rc.1`, that exact tag
is added. No workflow creates `latest`, major, minor, build-metadata, or any
other moving tag. The repository does not create the first semantic-version tag
until SHA publishing and anonymous pulls have been verified.

Deploy by manifest digest, never by tag:

```bash
docker pull ghcr.io/cthierer/canterbury-mcp-server@sha256:<manifest-digest>
```

For rollback, retrieve the prior verified digest from the workflow's
`image-digests-<commit>` artifact and pin the deployment to that digest. Each
publish prints the digests, writes them to the workflow summary, and uploads
the same JSON artifact.

## Local Interface

`make publish-images` requires a local Git `REF` resolving to the currently
checked-out commit. It never accepts credentials as arguments. Authenticate
separately, using stdin when a push is intended:

```bash
printf '%s' "$GITHUB_TOKEN" | docker login ghcr.io --username "$GITHUB_ACTOR" --password-stdin
```

The supported inputs are:

| Input   | Values                                          | Default   |
| ------- | ----------------------------------------------- | --------- |
| `REF`   | Required local Git ref or full commit SHA       | None      |
| `IMAGE` | `all`, `mcp-server`, `vault-service`, or `sync` | `all`     |
| `MODE`  | `dry-run`, `build`, or `push`                   | `dry-run` |

Examples:

```bash
make publish-images REF=HEAD
MODE=build IMAGE=vault-service make publish-images REF=HEAD
MODE=push IMAGE=all make publish-images REF=refs/heads/main
```

`dry-run` validates the ref and prints the Bake graph without building or
contacting a registry. `build` builds local `linux/amd64` images with Docker's
local exporter and does not push. `push` is the only registry-writing mode; it
requires a clean worktree and a prior `docker login ghcr.io`.

All builds use the Git commit timestamp as BuildKit's `SOURCE_DATE_EPOCH` with
the `rewrite-timestamp=true` exporter option and install runtime packages from
a signed, immutable Debian Snapshot with exact direct package versions. This
makes a clean rebuild of the same revision use the same base manifests, package
inputs, and image and layer timestamps. See
[Dependency Maintenance](maintenance.md#debian-runtime-packages) before
changing those pins.

The script rejects empty references, `latest`, major/minor or other moving
release tags, build metadata, a ref that differs from `HEAD`, and commits with
more than one eligible exact release tag. It always creates the full SHA tag.

## CI Publishing And Verification

Pull requests run the quality checks, build all three images, and inspect their
amd64 platform, OCI labels, non-root user, entrypoint, and healthcheck. They do
not authenticate or push. Successful `main`, exact-version-tag, and authorized
manual-dispatch runs publish only after the checked-out commit has been proved
reachable from `main`; this prevents package credentials from being exposed to
arbitrary pull-request code. The workflow uses `contents: read` by default and
grants `packages: write` only to the publishing job.

Registry pushes attach BuildKit maximum-mode provenance and SPDX SBOM
attestations. Inspect a published digest and its referrers with tooling that
supports OCI artifacts, for example:

```bash
docker buildx imagetools inspect ghcr.io/cthierer/canterbury-mcp-server@sha256:<manifest-digest>
```

After the first workflow publish, GitHub creates each GHCR package as private.
The package owner must make each package public in its GitHub package settings,
then verify an anonymous digest pull from a host without GHCR credentials:

```bash
docker logout ghcr.io
docker pull ghcr.io/cthierer/canterbury-mcp-server@sha256:<manifest-digest>
```

Do not hand a tag to deployment automation. Once these pulls and attached
attestations have been verified, hand the three manifest digests to the
personal-cloud deployment work without editing that repository as part of this
issue.
