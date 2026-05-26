package devauthjwt

import "errors"

// ErrInvalidIssuer indicates the minter was configured without a usable issuer.
var ErrInvalidIssuer = errors.New("invalid issuer claim value")
