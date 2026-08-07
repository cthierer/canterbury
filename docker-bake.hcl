// Canonical production image build graph. The publish script supplies immutable
// tags, the checked-out revision, and its commit timestamp.

variable "VERSION" {
  default = "dev"
}

variable "REVISION" {
  default = "unknown"
}

variable "CREATED" {
  default = "1970-01-01T00:00:00Z"
}

variable "REGISTRY" {
  default = "ghcr.io/cthierer"
}

group "default" {
  targets = ["mcp-server", "vault-service", "sync"]
}

target "common" {
  context = "."
  platforms = ["linux/amd64"]

  args = {
    CANTERBURY_VERSION = "${VERSION}"
    CANTERBURY_REVISION = "${REVISION}"
    CANTERBURY_CREATED = "${CREATED}"
  }

  labels = {
    "org.opencontainers.image.created" = "${CREATED}"
    "org.opencontainers.image.licenses" = "MIT"
    "org.opencontainers.image.revision" = "${REVISION}"
    "org.opencontainers.image.source" = "https://github.com/cthierer/canterbury"
    "org.opencontainers.image.version" = "${VERSION}"
  }

  attest = ["type=provenance,mode=max", "type=sbom"]
}

target "mcp-server" {
  inherits = ["common"]
  dockerfile = "Dockerfile.mcp-server"
  tags = ["${REGISTRY}/canterbury-mcp-server:${VERSION}"]
  labels = {
    "org.opencontainers.image.description" = "Canterbury MCP server"
    "org.opencontainers.image.title" = "Canterbury MCP Server"
  }
}

target "vault-service" {
  inherits = ["common"]
  dockerfile = "Dockerfile.vault-service"
  tags = ["${REGISTRY}/canterbury-vault-service:${VERSION}"]
  labels = {
    "org.opencontainers.image.description" = "Canterbury scoped vault service"
    "org.opencontainers.image.title" = "Canterbury Vault Service"
  }
}

target "sync" {
  inherits = ["common"]
  dockerfile = "Dockerfile.sync"
  tags = ["${REGISTRY}/canterbury-sync:${VERSION}"]
  labels = {
    "org.opencontainers.image.description" = "Canterbury Obsidian sync worker"
    "org.opencontainers.image.title" = "Canterbury Sync Worker"
  }
}
