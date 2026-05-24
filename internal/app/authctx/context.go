// Package authctx carries authenticated principals through request contexts.
package authctx

import (
	"context"

	"github.com/cthierer/canterbury/internal/app/auth"
)

type key struct{}

// WithPrincipal stores an authenticated principal on ctx.
func WithPrincipal(ctx context.Context, principal auth.Principal) context.Context {
	return context.WithValue(ctx, key{}, principal)
}

// PrincipalFromContext returns the authenticated principal stored on ctx.
func PrincipalFromContext(ctx context.Context) (auth.Principal, bool) {
	principal, ok := ctx.Value(key{}).(auth.Principal)
	return principal, ok
}
