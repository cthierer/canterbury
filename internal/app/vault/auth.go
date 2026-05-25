package vault

import (
	"context"

	appauth "github.com/cthierer/canterbury/internal/app/auth"
	"github.com/cthierer/canterbury/internal/app/authctx"
	domainauth "github.com/cthierer/canterbury/internal/domain/auth"
)

func principalFromContext(ctx context.Context) (appauth.Principal, error) {
	principal, ok := authctx.PrincipalFromContext(ctx)
	if !ok {
		return appauth.Principal{}, domainauth.ErrMissingPrincipal
	}

	return principal, nil
}
