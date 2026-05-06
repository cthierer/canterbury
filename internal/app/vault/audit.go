package vault

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cthierer/canterbury/internal/app/auditctx"
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

func (s *Service) createEvent(ctx context.Context, occurredAt time.Time) audit.Event {
	metadata, ok := auditctx.MetadataFromContext(ctx)

	actor := metadata.Actor
	if !ok {
		actor = audit.Actor{
			Issuer: "self",
			Scopes: s.principal.Scopes,
		}
	}

	client := metadata.Client
	if !ok {
		client = audit.Client{
			Interface: audit.ClientInterfaceService,
		}
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
