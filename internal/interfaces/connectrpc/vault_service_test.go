package connectrpc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	vaultv1 "github.com/cthierer/canterbury/gen/go/canterbury/vault/v1"
	appvault "github.com/cthierer/canterbury/internal/app/vault"
	domainvault "github.com/cthierer/canterbury/internal/domain/vault"
)

func TestNewVaultServiceHandler(t *testing.T) {
	t.Run("requires vault application", func(t *testing.T) {
		_, err := NewVaultServiceHandler(nil)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("creates handler", func(t *testing.T) {
		handler, err := NewVaultServiceHandler(&fakeVaultApplication{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if handler == nil {
			t.Fatal("expected handler")
		}
	})
}

func TestVaultServiceHandlerReadNote(t *testing.T) {
	t.Run("returns note", func(t *testing.T) {
		notePath := mustNotePath(t, "Projects/Canterbury.md")
		modifiedAt := time.Date(2026, 4, 29, 14, 30, 15, 123, time.UTC)
		handler := mustHandler(t, &fakeVaultApplication{
			readNoteFunc: func(_ context.Context, path domainvault.NotePath) (domainvault.Note, error) {
				if path != notePath {
					t.Fatalf("got path %q, want %q", path, notePath)
				}

				return domainvault.Note{
					Ref: domainvault.NoteRef{
						Path:  notePath,
						Title: "Canterbury",
					},
					Metadata: domainvault.NoteMetadata{
						Path:       notePath,
						Title:      "Canterbury",
						Tags:       []domainvault.Tag{"project", "ai"},
						SizeBytes:  128,
						ModifiedAt: modifiedAt,
						Frontmatter: map[string]any{
							"summary": "Public summary",
							"created": modifiedAt,
						},
					},
					Content: "# Canterbury\n",
				}, nil
			},
		})

		resp, err := handler.ReadNote(context.Background(), connect.NewRequest(&vaultv1.ReadNoteRequest{
			Ref: &vaultv1.NoteRef{
				Path: notePath.String(),
			},
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		note := resp.Msg.Note
		if note.GetRef().GetPath() != notePath.String() {
			t.Fatalf("got path %q, want %q", note.GetRef().GetPath(), notePath)
		}

		if note.GetContent() != "# Canterbury\n" {
			t.Fatalf("got content %q, want note content", note.GetContent())
		}

		metadata := note.GetMetadata()
		if metadata.GetTitle() != "Canterbury" {
			t.Fatalf("got title %q, want Canterbury", metadata.GetTitle())
		}

		if metadata.GetSizeBytes() != 128 {
			t.Fatalf("got size %d, want 128", metadata.GetSizeBytes())
		}

		if !metadata.GetModifiedAt().AsTime().Equal(modifiedAt) {
			t.Fatalf("got modified_at %v, want %v", metadata.GetModifiedAt().AsTime(), modifiedAt)
		}

		tags := metadata.GetTags()
		if len(tags) != 2 || tags[0] != "project" || tags[1] != "ai" {
			t.Fatalf("got tags %#v, want project and ai", tags)
		}

		properties := metadata.GetProperties().AsMap()
		if properties["summary"] != "Public summary" {
			t.Fatalf("got summary %#v, want Public summary", properties["summary"])
		}

		if properties["created"] != modifiedAt.Format(time.RFC3339Nano) {
			t.Fatalf("got created %#v, want timestamp string", properties["created"])
		}
	})

	t.Run("rejects missing ref", func(t *testing.T) {
		handler := mustHandler(t, &fakeVaultApplication{})

		_, err := handler.ReadNote(context.Background(), connect.NewRequest(&vaultv1.ReadNoteRequest{}))
		assertConnectCode(t, err, connect.CodeInvalidArgument)
	})

	t.Run("rejects invalid path", func(t *testing.T) {
		handler := mustHandler(t, &fakeVaultApplication{})

		_, err := handler.ReadNote(context.Background(), connect.NewRequest(&vaultv1.ReadNoteRequest{
			Ref: &vaultv1.NoteRef{
				Path: "../Secrets.md",
			},
		}))
		assertConnectCode(t, err, connect.CodeInvalidArgument)
	})

	tests := []struct {
		name    string
		readErr error
		want    connect.Code
	}{
		{
			name:    "maps not found",
			readErr: fmt.Errorf("repository read: %w", domainvault.ErrNoteNotFound),
			want:    connect.CodeNotFound,
		},
		{
			name:    "maps permission denied",
			readErr: fmt.Errorf("authorize note: %w", appvault.ErrPermissionDenied),
			want:    connect.CodePermissionDenied,
		},
		{
			name:    "maps invalid note path",
			readErr: fmt.Errorf("repository read: %w", domainvault.ErrInvalidNotePath),
			want:    connect.CodeInvalidArgument,
		},
		{
			name:    "maps unavailable vault",
			readErr: fmt.Errorf("repository read: %w", domainvault.ErrVaultUnavailable),
			want:    connect.CodeUnavailable,
		},
		{
			name:    "maps cancellation",
			readErr: context.Canceled,
			want:    connect.CodeCanceled,
		},
		{
			name:    "maps deadline",
			readErr: context.DeadlineExceeded,
			want:    connect.CodeDeadlineExceeded,
		},
		{
			name:    "maps unknown error",
			readErr: errors.New("boom"),
			want:    connect.CodeUnknown,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := mustHandler(t, &fakeVaultApplication{
				readNoteFunc: func(context.Context, domainvault.NotePath) (domainvault.Note, error) {
					return domainvault.Note{}, test.readErr
				},
			})

			_, err := handler.ReadNote(context.Background(), readNoteRequest("Projects/Canterbury.md"))
			assertConnectCode(t, err, test.want)
		})
	}

	t.Run("maps frontmatter formatting failure", func(t *testing.T) {
		notePath := mustNotePath(t, "Projects/Canterbury.md")
		handler := mustHandler(t, &fakeVaultApplication{
			readNoteFunc: func(context.Context, domainvault.NotePath) (domainvault.Note, error) {
				return domainvault.Note{
					Ref: domainvault.NoteRef{Path: notePath},
					Metadata: domainvault.NoteMetadata{
						Path: notePath,
						Frontmatter: map[string]any{
							"bad": map[int]string{1: "one"},
						},
					},
				}, nil
			},
		})

		_, err := handler.ReadNote(context.Background(), readNoteRequest(notePath.String()))
		assertConnectCode(t, err, connect.CodeUnknown)
	})
}

func TestFromMapNormalizesYAMLValues(t *testing.T) {
	timestamp := time.Date(2026, 4, 29, 14, 30, 15, 123, time.UTC)

	properties, err := fromMap(map[string]any{
		"created": timestamp,
		"aliases": []string{
			"alpha",
			"beta",
		},
		"nested": map[string]any{
			"due": timestamp,
			"counts": []int{
				1,
				2,
			},
		},
		"typed_map": map[string]string{
			"status": "ready",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := properties.AsMap()
	wantTimestamp := timestamp.Format(time.RFC3339Nano)
	if got["created"] != wantTimestamp {
		t.Fatalf("got created %#v, want %q", got["created"], wantTimestamp)
	}

	aliases, ok := got["aliases"].([]any)
	if !ok {
		t.Fatalf("got aliases type %T, want []any", got["aliases"])
	}
	if len(aliases) != 2 || aliases[0] != "alpha" || aliases[1] != "beta" {
		t.Fatalf("got aliases %#v, want alpha and beta", aliases)
	}

	nested, ok := got["nested"].(map[string]any)
	if !ok {
		t.Fatalf("got nested type %T, want map[string]any", got["nested"])
	}
	if nested["due"] != wantTimestamp {
		t.Fatalf("got nested due %#v, want %q", nested["due"], wantTimestamp)
	}

	counts, ok := nested["counts"].([]any)
	if !ok {
		t.Fatalf("got counts type %T, want []any", nested["counts"])
	}
	if len(counts) != 2 || counts[0] != float64(1) || counts[1] != float64(2) {
		t.Fatalf("got counts %#v, want 1 and 2", counts)
	}

	typedMap, ok := got["typed_map"].(map[string]any)
	if !ok {
		t.Fatalf("got typed map type %T, want map[string]any", got["typed_map"])
	}
	if typedMap["status"] != "ready" {
		t.Fatalf("got typed map status %#v, want ready", typedMap["status"])
	}
}

func TestFromMapRejectsNonStringMapKeys(t *testing.T) {
	_, err := fromMap(map[string]any{
		"bad": map[int]string{
			1: "one",
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "unsupported map key type int") {
		t.Fatalf("got error %q, want unsupported map key type", err)
	}
}

type fakeVaultApplication struct {
	readNoteFunc    func(context.Context, domainvault.NotePath) (domainvault.Note, error)
	searchNotesFunc func(context.Context, domainvault.SearchNotesQuery) (domainvault.SearchNotesPage, error)
}

func (v *fakeVaultApplication) ReadNote(
	ctx context.Context,
	path domainvault.NotePath,
) (domainvault.Note, error) {
	if v.readNoteFunc == nil {
		return domainvault.Note{}, errors.New("read note not implemented")
	}

	return v.readNoteFunc(ctx, path)
}

func (v *fakeVaultApplication) SearchNotes(
	ctx context.Context,
	query domainvault.SearchNotesQuery,
) (domainvault.SearchNotesPage, error) {
	if v.searchNotesFunc == nil {
		return domainvault.SearchNotesPage{}, errors.New("search notes not implemented")
	}

	return v.searchNotesFunc(ctx, query)
}

func mustHandler(t *testing.T, vault VaultApplication) *VaultServiceHandler {
	t.Helper()

	handler, err := NewVaultServiceHandler(vault)
	if err != nil {
		t.Fatalf("create handler: %v", err)
	}

	return handler
}

func mustNotePath(t *testing.T, value string) domainvault.NotePath {
	t.Helper()

	notePath, err := domainvault.NewNotePath(value)
	if err != nil {
		t.Fatalf("parse note path: %v", err)
	}

	return notePath
}

func readNoteRequest(path string) *connect.Request[vaultv1.ReadNoteRequest] {
	return connect.NewRequest(&vaultv1.ReadNoteRequest{
		Ref: &vaultv1.NoteRef{
			Path: path,
		},
	})
}

func assertConnectCode(t *testing.T, err error, want connect.Code) {
	t.Helper()

	if err == nil {
		t.Fatalf("got nil error, want %s", want)
	}

	if got := connect.CodeOf(err); got != want {
		t.Fatalf("got code %s, want %s: %v", got, want, err)
	}
}
