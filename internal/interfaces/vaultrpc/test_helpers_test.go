package connectrpc

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	vaultv1 "github.com/cthierer/canterbury/gen/go/canterbury/vault/v1"
	domainvault "github.com/cthierer/canterbury/internal/domain/vault"
)

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

func mustNotePath(t *testing.T) domainvault.NotePath {
	t.Helper()

	const value = "Projects/Canterbury.md"

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
