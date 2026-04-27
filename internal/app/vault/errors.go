package vault

import "errors"

// ErrPermissionDenied indicates the principal may not access a vault resource.
var ErrPermissionDenied = errors.New("permission denied")
