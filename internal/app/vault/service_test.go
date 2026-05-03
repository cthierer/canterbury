package vault_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/cthierer/canterbury/internal/app/auth"
	appvault "github.com/cthierer/canterbury/internal/app/vault"
	"github.com/cthierer/canterbury/internal/domain/audit"
	domain "github.com/cthierer/canterbury/internal/domain/vault"
)

func TestNewService(t *testing.T) {
	t.Run("requires repository", func(t *testing.T) {
		_, err := appvault.NewService(nil, testPrincipal(), &fakeAuditLogger{})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("requires principal scopes", func(t *testing.T) {
		_, err := appvault.NewService(&fakeRepository{}, auth.Principal{}, &fakeAuditLogger{})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("requires audit log", func(t *testing.T) {
		_, err := appvault.NewService(&fakeRepository{}, testPrincipal(), nil)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("creates service", func(t *testing.T) {
		service, err := appvault.NewService(&fakeRepository{}, testPrincipal(), &fakeAuditLogger{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if service == nil {
			t.Fatal("expected service")
		}
	})
}

func TestServiceReadNote(t *testing.T) {
	t.Run("allows matching scope and sanitizes frontmatter", func(t *testing.T) {
		notePath := mustNotePath(t, "Projects/Canterbury.md")
		auditLog := &fakeAuditLogger{}
		repository := &fakeRepository{
			readNoteFunc: func(_ context.Context, path domain.NotePath) (domain.Note, error) {
				if path != notePath {
					t.Fatalf("got path %q, want %q", path, notePath)
				}

				return noteWithAccess(notePath, []domain.Scope{"personal-agent"}), nil
			},
		}
		service := mustServiceWithAuditLog(t, repository, auditLog)

		note, err := service.ReadNote(context.Background(), notePath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		assertPublicFrontmatter(t, note.Metadata.Frontmatter)
		assertRecordedEvent(t, auditLog, audit.EventTypeVaultReadAllowed)
		if auditLog.events[0].Outcome.Status != audit.OutcomeStatusSuccess {
			t.Fatalf("got outcome status %q, want %q", auditLog.events[0].Outcome.Status, audit.OutcomeStatusSuccess)
		}
		if auditLog.events[0].Policy.Decision != audit.PolicyDecisionAllow {
			t.Fatalf("got policy decision %q, want %q", auditLog.events[0].Policy.Decision, audit.PolicyDecisionAllow)
		}
	})

	t.Run("denies missing matching scope", func(t *testing.T) {
		notePath := mustNotePath(t, "Projects/Private.md")
		auditLog := &fakeAuditLogger{}
		repository := &fakeRepository{
			readNoteFunc: func(context.Context, domain.NotePath) (domain.Note, error) {
				return noteWithAccess(notePath, []domain.Scope{"other-agent"}), nil
			},
		}
		service := mustServiceWithAuditLog(t, repository, auditLog)

		_, err := service.ReadNote(context.Background(), notePath)
		if !errors.Is(err, appvault.ErrPermissionDenied) {
			t.Fatalf("got error %v, want %v", err, appvault.ErrPermissionDenied)
		}

		assertRecordedEvent(t, auditLog, audit.EventTypeVaultReadDenied)
		if auditLog.events[0].Outcome.Code != audit.OutcomeCodePermissionDenied {
			t.Fatalf("got outcome code %q, want %q", auditLog.events[0].Outcome.Code, audit.OutcomeCodePermissionDenied)
		}
		if auditLog.events[0].Policy.Decision != audit.PolicyDecisionDeny {
			t.Fatalf("got policy decision %q, want %q", auditLog.events[0].Policy.Decision, audit.PolicyDecisionDeny)
		}
	})

	t.Run("records repository failure", func(t *testing.T) {
		notePath := mustNotePath(t, "Projects/Missing.md")
		auditLog := &fakeAuditLogger{}
		repository := &fakeRepository{
			readNoteFunc: func(context.Context, domain.NotePath) (domain.Note, error) {
				return domain.Note{}, domain.ErrNoteNotFound
			},
		}
		service := mustServiceWithAuditLog(t, repository, auditLog)

		_, err := service.ReadNote(context.Background(), notePath)
		if !errors.Is(err, domain.ErrNoteNotFound) {
			t.Fatalf("got error %v, want %v", err, domain.ErrNoteNotFound)
		}

		assertRecordedEvent(t, auditLog, audit.EventTypeVaultReadFailed)
		if auditLog.events[0].Outcome.Code != audit.OutcomeCodeNotFound {
			t.Fatalf("got outcome code %q, want %q", auditLog.events[0].Outcome.Code, audit.OutcomeCodeNotFound)
		}
	})

	t.Run("fails closed when audit logging fails", func(t *testing.T) {
		notePath := mustNotePath(t, "Projects/Canterbury.md")
		auditErr := errors.New("audit write failed")
		repository := &fakeRepository{
			readNoteFunc: func(context.Context, domain.NotePath) (domain.Note, error) {
				return noteWithAccess(notePath, []domain.Scope{"personal-agent"}), nil
			},
		}
		service := mustServiceWithAuditLog(t, repository, &fakeAuditLogger{err: auditErr})

		_, err := service.ReadNote(context.Background(), notePath)
		if !errors.Is(err, auditErr) {
			t.Fatalf("got error %v, want %v", err, auditErr)
		}
	})
}

func TestServiceSearchNotes(t *testing.T) {
	notePath := mustNotePath(t, "Projects/Canterbury.md")
	auditLog := &fakeAuditLogger{}
	var gotQuery domain.SearchNotesQuery
	repository := &fakeRepository{
		searchNotesFunc: func(_ context.Context, query domain.SearchNotesQuery) (domain.SearchNotesPage, error) {
			gotQuery = query

			return domain.SearchNotesPage{
				Results: []domain.SearchNoteResult{
					{
						Ref:      domain.NoteRef{Path: notePath, Title: "Canterbury"},
						Metadata: noteWithAccess(notePath, []domain.Scope{"personal-agent"}).Metadata,
						Snippet:  "Canterbury notes",
					},
				},
				NextCursor: "1",
			}, nil
		},
	}
	service := mustServiceWithAuditLog(t, repository, auditLog)
	page, err := service.SearchNotes(context.Background(), domain.SearchNotesQuery{
		Text: domain.TextSearch{
			Terms: []string{"canterbury"},
		},
		Access: domain.AccessFilter{
			ScopesAll: []domain.Scope{"caller-controlled"},
			ScopesAny: []domain.Scope{"caller-controlled"},
		},
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantScopes := []domain.Scope{"personal-agent"}
	if !reflect.DeepEqual(gotQuery.Access.ScopesAny, wantScopes) {
		t.Fatalf("got access scopes %#v, want %#v", gotQuery.Access.ScopesAny, wantScopes)
	}

	if len(gotQuery.Access.ScopesAll) != 0 {
		t.Fatalf("got caller-controlled required scopes %#v, want none", gotQuery.Access.ScopesAll)
	}

	if page.NextCursor != "1" {
		t.Fatalf("got next cursor %q, want %q", page.NextCursor, "1")
	}

	if len(page.Results) != 1 {
		t.Fatalf("got %d results, want 1", len(page.Results))
	}

	assertPublicFrontmatter(t, page.Results[0].Metadata.Frontmatter)
	assertRecordedEvent(t, auditLog, audit.EventTypeVaultSearchCompleted)
	if auditLog.events[0].Outcome.Status != audit.OutcomeStatusSuccess {
		t.Fatalf("got outcome status %q, want %q", auditLog.events[0].Outcome.Status, audit.OutcomeStatusSuccess)
	}
	if auditLog.events[0].Policy.Decision != audit.PolicyDecisionAllow {
		t.Fatalf("got policy decision %q, want %q", auditLog.events[0].Policy.Decision, audit.PolicyDecisionAllow)
	}
}

func TestServiceSearchNotesRecordsRepositoryFailure(t *testing.T) {
	auditLog := &fakeAuditLogger{}
	repository := &fakeRepository{
		searchNotesFunc: func(context.Context, domain.SearchNotesQuery) (domain.SearchNotesPage, error) {
			return domain.SearchNotesPage{}, domain.ErrInvalidSearch
		},
	}
	service := mustServiceWithAuditLog(t, repository, auditLog)

	_, err := service.SearchNotes(context.Background(), domain.SearchNotesQuery{
		Sort: domain.SearchSort("unknown"),
	})
	if !errors.Is(err, domain.ErrInvalidSearch) {
		t.Fatalf("got error %v, want %v", err, domain.ErrInvalidSearch)
	}

	assertRecordedEvent(t, auditLog, audit.EventTypeVaultSearchFailed)
	if auditLog.events[0].Outcome.Code != audit.OutcomeCodeInvalidArgument {
		t.Fatalf("got outcome code %q, want %q", auditLog.events[0].Outcome.Code, audit.OutcomeCodeInvalidArgument)
	}
}

func TestServiceSearchNotesFailsClosedWhenAuditLoggingFails(t *testing.T) {
	notePath := mustNotePath(t, "Projects/Canterbury.md")
	auditErr := errors.New("audit write failed")
	repository := &fakeRepository{
		searchNotesFunc: func(context.Context, domain.SearchNotesQuery) (domain.SearchNotesPage, error) {
			return domain.SearchNotesPage{
				Results: []domain.SearchNoteResult{
					{
						Ref:      domain.NoteRef{Path: notePath, Title: "Canterbury"},
						Metadata: noteWithAccess(notePath, []domain.Scope{"personal-agent"}).Metadata,
						Snippet:  "Canterbury notes",
					},
				},
			}, nil
		},
	}
	service := mustServiceWithAuditLog(t, repository, &fakeAuditLogger{err: auditErr})

	_, err := service.SearchNotes(context.Background(), domain.SearchNotesQuery{})
	if !errors.Is(err, auditErr) {
		t.Fatalf("got error %v, want %v", err, auditErr)
	}
}

type fakeRepository struct {
	readNoteFunc    func(context.Context, domain.NotePath) (domain.Note, error)
	searchNotesFunc func(context.Context, domain.SearchNotesQuery) (domain.SearchNotesPage, error)
}

func (r *fakeRepository) ReadNote(ctx context.Context, path domain.NotePath) (domain.Note, error) {
	if r.readNoteFunc == nil {
		return domain.Note{}, errors.New("read note not implemented")
	}

	return r.readNoteFunc(ctx, path)
}

func (r *fakeRepository) SearchNotes(
	ctx context.Context,
	query domain.SearchNotesQuery,
) (domain.SearchNotesPage, error) {
	if r.searchNotesFunc == nil {
		return domain.SearchNotesPage{}, errors.New("search notes not implemented")
	}

	return r.searchNotesFunc(ctx, query)
}

func mustServiceWithAuditLog(
	t *testing.T,
	repository domain.Repository,
	auditLog *fakeAuditLogger,
) *appvault.Service {
	t.Helper()

	service, err := appvault.NewService(repository, testPrincipal(), auditLog)
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	return service
}

type fakeAuditLogger struct {
	events []audit.Event
	err    error
}

func (l *fakeAuditLogger) RecordEvent(_ context.Context, event audit.Event) error {
	l.events = append(l.events, event)
	return l.err
}

func assertRecordedEvent(t *testing.T, logger *fakeAuditLogger, eventType audit.EventType) {
	t.Helper()

	if len(logger.events) != 1 {
		t.Fatalf("got %d audit events, want 1", len(logger.events))
	}

	if got := logger.events[0].Type(); got != eventType {
		t.Fatalf("got event type %q, want %q", got, eventType)
	}
}

func testPrincipal() auth.Principal {
	return auth.Principal{Scopes: []domain.Scope{"personal-agent"}}
}

func noteWithAccess(notePath domain.NotePath, scopes []domain.Scope) domain.Note {
	return domain.Note{
		Ref: domain.NoteRef{
			Path:  notePath,
			Title: "Canterbury",
		},
		Metadata: domain.NoteMetadata{
			Path:           notePath,
			Title:          "Canterbury",
			Access:         domain.ResourceAccess{Scopes: scopes},
			HasFrontmatter: true,
			Frontmatter: map[string]any{
				"access": map[string]any{
					"scopes": []any{"personal-agent"},
				},
				"Access":  "case-insensitive reserved key",
				"summary": "Public summary",
			},
		},
		Content: "Canterbury notes",
	}
}

func assertPublicFrontmatter(t *testing.T, frontmatter map[string]any) {
	t.Helper()

	if _, ok := frontmatter["access"]; ok {
		t.Fatal("frontmatter exposes reserved access key")
	}

	if _, ok := frontmatter["Access"]; ok {
		t.Fatal("frontmatter exposes reserved Access key")
	}

	if got := frontmatter["summary"]; got != "Public summary" {
		t.Fatalf("got summary %#v, want %q", got, "Public summary")
	}
}

func mustNotePath(t *testing.T, value string) domain.NotePath {
	t.Helper()

	notePath, err := domain.NewNotePath(value)
	if err != nil {
		t.Fatalf("parse note path: %v", err)
	}

	return notePath
}
