package vaultfs

import (
	"fmt"
	"slices"
	"strings"

	"github.com/cthierer/canterbury/internal/domain/vault"
)

type sorter func(results []vault.SearchNoteResult)

func sortByQuery(query vault.SearchNotesQuery) (sorter, error) {
	switch query.NormalizedSort() {
	case vault.SearchSortModifiedDesc:
		return sortByModifiedDesc, nil
	case vault.SearchSortPathAsc:
		return sortByPathAsc, nil
	default:
		return nil, fmt.Errorf("sort by query %q: %w", query.Sort, vault.ErrInvalidSearch)
	}
}

func compareResultModifiedDesc(a, b vault.SearchNoteResult) int {
	if a.Metadata.ModifiedAt.After(b.Metadata.ModifiedAt) {
		return -1
	}

	if a.Metadata.ModifiedAt.Before(b.Metadata.ModifiedAt) {
		return 1
	}

	return compareResultPathAsc(a, b)
}

func sortByModifiedDesc(results []vault.SearchNoteResult) {
	slices.SortFunc(results, compareResultModifiedDesc)
}

func compareResultPathAsc(a, b vault.SearchNoteResult) int {
	return strings.Compare(a.Ref.Path.String(), b.Ref.Path.String())
}

func sortByPathAsc(results []vault.SearchNoteResult) {
	slices.SortFunc(results, compareResultPathAsc)
}
