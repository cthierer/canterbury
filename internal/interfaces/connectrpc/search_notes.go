package connectrpc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"connectrpc.com/connect"
	vaultv1 "github.com/cthierer/canterbury/gen/go/canterbury/vault/v1"
	domainvault "github.com/cthierer/canterbury/internal/domain/vault"
)

// SearchNotes handles search notes requests.
func (h *VaultServiceHandler) SearchNotes(
	ctx context.Context,
	req *connect.Request[vaultv1.SearchNotesRequest],
) (*connect.Response[vaultv1.SearchNotesResponse], error) {
	query, err := protoToSearchNotesQuery(req.Msg)
	if err != nil {
		slog.DebugContext(ctx, "failed to convert search request", "err", err)
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid query; please check and try again"))
	}

	page, err := h.vault.SearchNotes(ctx, query)
	if err != nil {
		connectErr := classifySearchNotesError(req.Msg, err)
		logConnectError(ctx, "encountered an error searching vault", err, connectErr)
		return nil, connectErr
	}

	resultsMsg, err := searchResultsToProto(page.Results)
	if err != nil {
		slog.ErrorContext(ctx, "encountered an error building results messages", "err", err)
		return nil, connect.NewError(connect.CodeUnknown, errors.New("unexpected error building results"))
	}

	resp := connect.NewResponse(&vaultv1.SearchNotesResponse{
		Results:       resultsMsg,
		NextPageToken: page.NextCursor,
	})

	return resp, nil
}

func classifySearchNotesError(_ *vaultv1.SearchNotesRequest, err error) error {
	if errors.Is(err, domainvault.ErrInvalidSearch) {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("invalid search query"))
	}

	return classifySystemError(err)
}

func protoToSearchNotesQuery(msg *vaultv1.SearchNotesRequest) (domainvault.SearchNotesQuery, error) {
	notesQuery := domainvault.SearchNotesQuery{}

	if err := applyQuery(&notesQuery, msg.Query); err != nil {
		return notesQuery, fmt.Errorf("apply query: %w", err)
	}

	if err := applyFilter(&notesQuery, msg.Filter); err != nil {
		return notesQuery, fmt.Errorf("apply filter: %w", err)
	}

	if err := applySort(&notesQuery, msg.Sort); err != nil {
		return notesQuery, fmt.Errorf("apply sort: %w", err)
	}

	notesQuery.Cursor = msg.PageToken
	notesQuery.Limit = int(msg.PageSize)

	return notesQuery, nil
}

func applyQuery(query *domainvault.SearchNotesQuery, queryMsg *vaultv1.SearchNotesQuery) (err error) {
	if queryMsg == nil {
		return
	}

	term := strings.TrimSpace(queryMsg.Text)
	if term == "" {
		return
	}

	query.Text = domainvault.TextSearch{Terms: []string{term}}

	return
}

func applyFilter(query *domainvault.SearchNotesQuery, filterMsg *vaultv1.SearchNotesFilter) (err error) {
	if filterMsg == nil {
		return
	}

	query.Path.IncludePrefixes = filterMsg.IncludePathPrefixes
	query.Path.ExcludePrefixes = filterMsg.ExcludePathPrefixes

	query.Tags.All, err = stringsToTags(filterMsg.AllTags)
	if err != nil {
		return fmt.Errorf("build query tags \"all\": %w", err)
	}

	query.Tags.Any, err = stringsToTags(filterMsg.AnyTags)
	if err != nil {
		return fmt.Errorf("build query tags \"any\": %w", err)
	}

	return nil
}

func applySort(query *domainvault.SearchNotesQuery, sortMsg vaultv1.SearchSort) (err error) {
	switch sortMsg {
	case vaultv1.SearchSort_SEARCH_SORT_MODIFIED_DESC:
		query.Sort = domainvault.SearchSortModifiedDesc
	case vaultv1.SearchSort_SEARCH_SORT_PATH_ASC:
		query.Sort = domainvault.SearchSortPathAsc
	case vaultv1.SearchSort_SEARCH_SORT_UNSPECIFIED:
		query.Sort = domainvault.SearchSortPathAsc
	default:
		err = fmt.Errorf("unrecognized sort order: %q", sortMsg)
	}

	return
}

func stringsToTags(tagStrs []string) ([]domainvault.Tag, error) {
	tags := make([]domainvault.Tag, len(tagStrs))
	for i, value := range tagStrs {
		var err error

		tags[i], err = domainvault.NewTag(value)
		if err != nil {
			return nil, fmt.Errorf("parsing tag: %w", err)
		}
	}

	return tags, nil
}
