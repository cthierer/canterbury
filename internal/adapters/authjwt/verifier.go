package authjwt

import (
	"context"
	"fmt"
	"strings"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/cthierer/canterbury/internal/app/clock"
	"github.com/golang-jwt/jwt/v5"
)

// Verifier validates signed JWTs and returns trusted token claims.
type Verifier struct {
	clock   clock.Clock
	methods []string
	keyFunc jwt.Keyfunc
}

// NewVerifier creates a JWKS-backed JWT verifier.
func NewVerifier(ctx context.Context, jwksURL string, allowedMethods []string) (*Verifier, error) {
	jwksURL = strings.TrimSpace(jwksURL)
	if jwksURL == "" {
		return nil, fmt.Errorf("jwks url must not be blank")
	}

	methods := make([]string, len(allowedMethods))
	for i, method := range allowedMethods {
		method = strings.TrimSpace(method)
		if method == "" {
			return nil, fmt.Errorf("method must not be blank")
		}

		methods[i] = method
	}

	if len(methods) < 1 {
		return nil, fmt.Errorf("at least 1 method is required")
	}

	keyFunc, err := keyfunc.NewDefaultCtx(ctx, []string{jwksURL})
	if err != nil {
		return nil, fmt.Errorf("build JWKS key function: %w", err)
	}

	return &Verifier{
		clock:   clock.SystemClock{},
		methods: methods,
		keyFunc: keyFunc.Keyfunc,
	}, nil
}
