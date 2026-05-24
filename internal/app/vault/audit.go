package vault

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cthierer/canterbury/internal/app/auditctx"
	"github.com/cthierer/canterbury/internal/app/auth"
	"github.com/cthierer/canterbury/internal/domain/audit"
	domain "github.com/cthierer/canterbury/internal/domain/vault"
)

// AuditLogger records vault audit events.
type AuditLogger interface {
	RecordEvent(ctx context.Context, event audit.Event) error
}

type noteRef struct {
	Path  string `json:"path"`
	Title string `json:"title"`
}

func stringsFromScopes(scopes []domain.Scope) []string {
	scopeStrings := make([]string, 0, len(scopes))
	for _, scopeVal := range scopes {
		normalized := strings.TrimSpace(scopeVal.String())
		if normalized == "" {
			continue
		}

		scopeStrings = append(scopeStrings, normalized)
	}

	return scopeStrings
}

func (s *Service) recordEvent(ctx context.Context, event audit.Event) error {
	if err := s.auditLog.RecordEvent(ctx, event); err != nil {
		return fmt.Errorf("record audit log event: %w", err)
	}

	return nil
}

func createEvent(ctx context.Context, principal auth.Principal, occurredAt time.Time) audit.Event {
	metadata, ok := auditctx.MetadataFromContext(ctx)
	client := metadata.Client
	if !ok {
		client.Interface = audit.ClientInterfaceService
	}

	actor := audit.Actor{
		Issuer:      principal.Issuer,
		SubjectHash: principal.SubjectHash,
		Scopes:      principal.Scopes,
	}

	return audit.Event{
		OccurredAt: occurredAt.UTC(),
		RequestID:  metadata.RequestID,
		TraceID:    metadata.TraceID,
		Actor:      actor,
		Client:     client,
	}
}

func (s *Service) outcome(startedAt time.Time, status audit.OutcomeStatus, code audit.OutcomeCode) audit.Outcome {
	duration := s.clock.Now().Sub(startedAt)
	return audit.Outcome{
		Status:   status,
		Code:     code,
		Duration: duration,
	}
}
