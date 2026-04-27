// Package auth defines caller identity values used by application policy.
//
// The package does not authenticate transport requests itself. Protocol
// adapters, such as Connect RPC or future MCP interfaces, are responsible for
// resolving credentials or development configuration into principals that app
// services can authorize.
package auth
