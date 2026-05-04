package vault

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cthierer/canterbury/internal/domain/audit"
	domain "github.com/cthierer/canterbury/internal/domain/vault"
)

const (
	reasonRepositoryError  string = "repository_error"
	reasonInvalidNotePath  string = "invalid_note_path"
	reasonNoteNotFound     string = "note_not_found"
	reasonVaultUnavailable string = "vault_unavailable"
	reasonNoMatchingScope  string = "no_matching_scope"
)

type readNoteEventAllowedDetails struct {
	NoteRef        noteRef  `json:"note_ref"`
	ResourceScopes []string `json:"resource_scopes"`
	ContentBytes   int      `json:"content_bytes"`
}

func (readNoteEventAllowedDetails) EventType() audit.EventType {
	return audit.EventTypeVaultReadAllowed
}

func (s *Service) recordReadNoteAllowed(ctx context.Context, note domain.Note, startedAt time.Time) error {
	event := s.createEvent(ctx, startedAt)

	event.Outcome = s.outcome(
		startedAt,
		audit.OutcomeStatusSuccess,
		audit.OutcomeCodeOK,
	)

	event.Policy = audit.Policy{
		MatchedScopes: note.Metadata.Access.MatchedScopes(s.principal.Scopes),
		Decision:      audit.PolicyDecisionAllow,
	}

	event.Details = &readNoteEventAllowedDetails{
		NoteRef:        noteRef{Path: note.Ref.Path.String(), Title: note.Ref.Title},
		ResourceScopes: stringsFromScopes(note.Metadata.Access.Scopes),
		ContentBytes:   len(note.Content),
	}

	if err := s.recordEvent(ctx, event); err != nil {
		return fmt.Errorf("record read allowed event: %w", err)
	}

	return nil
}

type readNoteEventDeniedDetails struct {
	NoteRef        noteRef  `json:"note_ref"`
	ResourceScopes []string `json:"resource_scopes"`
	Reason         string   `json:"reason"`
}

func (readNoteEventDeniedDetails) EventType() audit.EventType {
	return audit.EventTypeVaultReadDenied
}

func (s *Service) recordReadNoteDenied(ctx context.Context, note domain.Note, startedAt time.Time) error {
	event := s.createEvent(ctx, startedAt)

	event.Outcome = s.outcome(
		startedAt,
		audit.OutcomeStatusFailed,
		audit.OutcomeCodePermissionDenied,
	)

	event.Policy = audit.Policy{
		Decision:      audit.PolicyDecisionDeny,
		MatchedScopes: []domain.Scope{},
	}

	event.Details = &readNoteEventDeniedDetails{
		NoteRef:        noteRef{Path: note.Ref.Path.String()},
		ResourceScopes: stringsFromScopes(note.Metadata.Access.Scopes),
		Reason:         reasonNoMatchingScope,
	}

	if err := s.recordEvent(ctx, event); err != nil {
		return fmt.Errorf("record read denied event: %w", err)
	}

	return nil
}

type readNoteEventErrorDetails struct {
	NoteRef noteRef `json:"note_ref"`
	Reason  string  `json:"reason"`
}

func (readNoteEventErrorDetails) EventType() audit.EventType {
	return audit.EventTypeVaultReadFailed
}

func (s *Service) recordReadNoteError(ctx context.Context, path domain.NotePath, err error, startedAt time.Time) error {
	event := s.createEvent(ctx, startedAt)
	status, code, reason := classifyReadNoteError(err)

	event.Outcome = s.outcome(startedAt, status, code)

	event.Details = &readNoteEventErrorDetails{
		NoteRef: noteRef{Path: path.String()},
		Reason:  reason,
	}

	if err := s.recordEvent(ctx, event); err != nil {
		return fmt.Errorf("record read error event: %w", err)
	}

	return nil
}

func classifyReadNoteError(err error) (audit.OutcomeStatus, audit.OutcomeCode, string) {
	switch {
	case errors.Is(err, domain.ErrInvalidNotePath):
		return audit.OutcomeStatusFailed, audit.OutcomeCodeInvalidArgument, reasonInvalidNotePath
	case errors.Is(err, domain.ErrNoteNotFound):
		return audit.OutcomeStatusFailed, audit.OutcomeCodeNotFound, reasonNoteNotFound
	case errors.Is(err, domain.ErrVaultUnavailable):
		return audit.OutcomeStatusError, audit.OutcomeCodeUnavailable, reasonVaultUnavailable
	default:
		return audit.OutcomeStatusError, audit.OutcomeCodeInternal, reasonRepositoryError
	}
}
