package connectrpc

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"connectrpc.com/connect"
	vaultv1 "github.com/cthierer/canterbury/gen/go/canterbury/vault/v1"
	appvault "github.com/cthierer/canterbury/internal/app/vault"
	domainvault "github.com/cthierer/canterbury/internal/domain/vault"
)

func TestVaultServiceHandlerReadNote(t *testing.T) {
	t.Run("returns note", func(t *testing.T) {
		notePath := mustNotePath(t)
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
						Path:  notePath,
						Title: "Canterbury",
					},
					Content: "# Canterbury\n",
				}, nil
			},
		})

		resp, err := handler.ReadNote(context.Background(), readNoteRequest(notePath.String()))
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

	t.Run("maps note conversion failure", func(t *testing.T) {
		notePath := mustNotePath(t)
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
