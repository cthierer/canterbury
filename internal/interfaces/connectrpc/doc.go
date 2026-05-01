// Package connectrpc adapts vault application use cases to Connect RPC handlers.
//
// The package translates between generated protocol buffer messages and domain
// vault types while leaving authorization and repository access decisions to the
// application layer.
package connectrpc
