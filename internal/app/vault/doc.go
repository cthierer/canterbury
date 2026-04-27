// Package vault implements application use cases for controlled vault access.
//
// The package coordinates repository reads and searches with caller policy. It
// accepts and returns domain vault types so protocol adapters, such as Connect
// RPC or future MCP tools, can translate their own wire shapes without owning
// authorization decisions.
package vault
