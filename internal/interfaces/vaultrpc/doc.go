// Package vaultrpc adapts vault application use cases to Connect RPC handlers.
//
// The package translates between generated protocol buffer messages and domain
// vault types while leaving authorization and repository access decisions to the
// application layer. It also provides transport interceptors for request-scoped
// audit metadata such as request IDs, trace IDs, client interface data, and
// hashed remote addresses.
package vaultrpc
