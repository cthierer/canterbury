package vaultfs

import (
	"context"
	"errors"
	"fmt"
	"os"
	pathpkg "path"

	"github.com/cthierer/canterbury/internal/domain/vault"
)

const defaultMaxNoteBytes = 10 * 1024 * 1024

// ReadNote reads one Markdown note from the filesystem vault mirror.
func (r *Repository) ReadNote(ctx context.Context, path vault.NotePath) (vault.Note, error) {
	if err := ctx.Err(); err != nil {
		return vault.Note{}, err
	}

	if isHiddenOrSystemPath(path) {
		return vault.Note{}, fmt.Errorf("reject hidden/system note path %q: %w", path, vault.ErrInvalidNotePath)
	}

	filePath, err := joinVaultPath(r.root, path)
	if err != nil {
		return vault.Note{}, fmt.Errorf("join vault path: %w", err)
	}

	info, err := os.Stat(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return vault.Note{}, fmt.Errorf("stat note %q: %w", filePath, vault.ErrNoteNotFound)
		}

		return vault.Note{}, fmt.Errorf("stat note %q: %w", filePath, err)
	}

	if info.IsDir() {
		return vault.Note{}, fmt.Errorf("note is directory %q: %w", filePath, ErrIsDirectory)
	}

	sizeBytes := info.Size()
	if sizeBytes > defaultMaxNoteBytes {
		return vault.Note{}, fmt.Errorf("stat note %q: %w", filePath, ErrFileTooLarge)
	}

	// #nosec G304 -- filePath is resolved by joinVaultPath, which rejects symlink escapes.
	content, err := os.ReadFile(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return vault.Note{}, fmt.Errorf("read note %q: %w", filePath, vault.ErrNoteNotFound)
		}

		return vault.Note{}, fmt.Errorf("read file %q: %w", filePath, err)
	}

	title := titleFromNotePath(path)

	document, err := parseNoteDocument(string(content))
	if err != nil {
		return vault.Note{}, fmt.Errorf("parse note document: %w", err)
	}

	return vault.Note{
		Content: document.Body,
		Metadata: vault.NoteMetadata{
			ModifiedAt:     info.ModTime(),
			Path:           path,
			SizeBytes:      sizeBytes,
			Title:          title,
			HasFrontmatter: document.HasFrontmatter,
			Frontmatter:    document.Frontmatter,
			Access:         document.Access,
			Tags:           document.Tags,
		},
		Ref: vault.NoteRef{
			Path:  path,
			Title: title,
		},
	}, nil
}

func titleFromNotePath(notePath vault.NotePath) string {
	fileName := pathpkg.Base(notePath.String())
	return fileName[:len(fileName)-len(pathpkg.Ext(fileName))]
}
