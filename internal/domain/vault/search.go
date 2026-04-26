package vault

// SearchSort names a deterministic ordering for note search results.
type SearchSort string

const (
	// SearchSortPathAsc orders notes by path from A to Z.
	SearchSortPathAsc SearchSort = "path_asc"

	// SearchSortModifiedDesc orders notes by newest modified timestamp first.
	SearchSortModifiedDesc SearchSort = "modified_desc"

	// DefaultSearchLimit is used when callers do not request a positive limit.
	DefaultSearchLimit = 50

	// DefaultSearchMaxLimit is the largest page size accepted by the domain.
	DefaultSearchMaxLimit = 200
)

// SearchNotesQuery describes text and metadata filters for note search.
type SearchNotesQuery struct {
	Text   TextSearch
	Path   PathFilter
	Tags   TagFilter
	Access AccessFilter
	Limit  int
	Cursor string
	Sort   SearchSort
}

// TextSearch filters notes by terms matched against searchable note text.
type TextSearch struct {
	Terms []string
}

// PathFilter includes or excludes notes by vault-relative path prefix.
type PathFilter struct {
	IncludePrefixes []string
	ExcludePrefixes []string
}

// TagFilter filters notes by required and optional tags.
type TagFilter struct {
	All []Tag
	Any []Tag
}

// AccessFilter filters notes by declared resource access scopes.
type AccessFilter struct {
	ScopesAll []Scope
	ScopesAny []Scope
}

// SearchNoteResult describes a matching note without returning full content.
type SearchNoteResult struct {
	Ref      NoteRef
	Metadata NoteMetadata
	Snippet  string
}

// SearchNotesPage contains one page of search results.
type SearchNotesPage struct {
	Results    []SearchNoteResult
	NextCursor string
}

// NormalizedLimit returns the effective bounded page size for the query.
func (q SearchNotesQuery) NormalizedLimit() int {
	if q.Limit <= 0 {
		return DefaultSearchLimit
	}

	if q.Limit > DefaultSearchMaxLimit {
		return DefaultSearchMaxLimit
	}

	return q.Limit
}

// NormalizedSort returns the requested sort or the default sort.
func (q SearchNotesQuery) NormalizedSort() SearchSort {
	if q.Sort == "" {
		return SearchSortPathAsc
	}

	return q.Sort
}
