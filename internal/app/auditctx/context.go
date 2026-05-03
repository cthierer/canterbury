package auditctx

import (
	"context"

	"github.com/cthierer/canterbury/internal/domain/audit"
)

type key struct{}

// Metadata carries request-scoped audit envelope fields through application
// calls.
type Metadata struct {
	RequestID audit.RequestID
	TraceID   audit.TraceID
	Actor     audit.Actor
	Client    audit.Client
}

// WithMetadata returns a context carrying audit metadata.
func WithMetadata(ctx context.Context, metadata Metadata) context.Context {
	return context.WithValue(ctx, key{}, metadata)
}

// MetadataFromContext returns audit metadata carried by ctx.
func MetadataFromContext(ctx context.Context) (Metadata, bool) {
	metadata, ok := ctx.Value(key{}).(Metadata)
	return metadata, ok
}
