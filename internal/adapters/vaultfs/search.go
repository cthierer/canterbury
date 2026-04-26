package vaultfs

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/cthierer/canterbury/internal/domain/vault"
)

const fileExtMarkdown = ".md"

// SearchNotes searches notes in the filesystem vault mirror.
func (r *Repository) SearchNotes(ctx context.Context, query vault.SearchNotesQuery) (vault.SearchNotesPage, error) {
	if err := ctx.Err(); err != nil {
		return vault.SearchNotesPage{}, err
	}

	matches := matchQuery(query)

	sortResults, err := sortByQuery(query)
	if err != nil {
		return vault.SearchNotesPage{}, fmt.Errorf("prepare search sort: %w", err)
	}

	paginateResults, err := paginateFromQuery(query)
	if err != nil {
		return vault.SearchNotesPage{}, fmt.Errorf("prepare search pagination: %w", err)
	}

	var results []vault.SearchNoteResult

	err = filepath.WalkDir(r.root, func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk %q: %w", filePath, walkErr)
		}

		if err := ctx.Err(); err != nil {
			return err
		}

		fileName := entry.Name()

		if filePath != r.root && entry.IsDir() {
			if shouldSkipDir(fileName) {
				return filepath.SkipDir
			}

			return nil
		}

		if shouldSkipFile(fileName) {
			return nil
		}

		relativePath, err := filepath.Rel(r.root, filePath)
		if err != nil {
			return fmt.Errorf("make vault-relative path %q: %w", filePath, err)
		}

		notePath, err := vault.NewNotePath(filepath.ToSlash(relativePath))
		if err != nil {
			// skip non-notes
			return nil
		}

		note, err := r.ReadNote(ctx, notePath)
		if err != nil {
			return fmt.Errorf("search notes: %w", err)
		}

		if !matches(note) {
			// skip notes that don't match
			return nil
		}

		result := vault.SearchNoteResult{
			Ref:      note.Ref,
			Metadata: note.Metadata,
			Snippet:  buildSnippet(note, query.Text),
		}

		results = append(results, result)

		return nil
	})
	if err != nil {
		return vault.SearchNotesPage{}, fmt.Errorf("scanning notes: %w", err)
	}

	sortResults(results)
	page := paginateResults(results)

	return page, nil
}

func shouldSkipDir(name string) bool {
	return isHiddenOrSystemName(name)
}

func shouldSkipFile(name string) bool {
	return isHiddenOrSystemName(name) || strings.ToLower(filepath.Ext(name)) != fileExtMarkdown
}
