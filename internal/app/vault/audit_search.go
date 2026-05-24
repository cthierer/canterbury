package vault

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cthierer/canterbury/internal/app/auth"
	"github.com/cthierer/canterbury/internal/domain/audit"
	domain "github.com/cthierer/canterbury/internal/domain/vault"
)

const reasonInvalidSearchQuery string = "invalid_search_query"

type searchNotesEventQueryDetails struct {
	TextHash            *string  `json:"text_hash"`
	HasText             bool     `json:"has_text"`
	IncludePathPrefixes []string `json:"include_path_prefixes"`
	ExcludePathPrefixes []string `json:"exclude_path_prefixes"`
	AllTags             []string `json:"all_tags"`
	AnyTags             []string `json:"any_tags"`
	PageSize            int      `json:"page_size"`
	PageTokenHash       *string  `json:"page_token_hash"`
}

type searchNotesEventResultsDetails struct {
	Count         int      `json:"count"`
	ReturnedRefs  []string `json:"returned_refs"`
	ResultSetHash string   `json:"result_set_hash"`
}

type searchNotesEventCompletedDetails struct {
	Query   searchNotesEventQueryDetails   `json:"query"`
	Results searchNotesEventResultsDetails `json:"results"`
}

func (searchNotesEventCompletedDetails) EventType() audit.EventType {
	return audit.EventTypeVaultSearchCompleted
}

func (s *Service) recordSearchNotesCompleted(
	ctx context.Context,
	principal auth.Principal,
	query domain.SearchNotesQuery,
	page domain.SearchNotesPage,
	startedAt time.Time,
) error {
	event := createEvent(ctx, principal, startedAt)

	event.Outcome = s.outcome(
		startedAt,
		audit.OutcomeStatusSuccess,
		audit.OutcomeCodeOK,
	)

	event.Policy = audit.Policy{
		MappingChecksum: principal.MappingChecksum,
		MatchedScopes:   matchedSearchResultScopes(page, principal.Scopes),
		Decision:        audit.PolicyDecisionAllow,
	}

	returnedRefs := returnedSearchResultRefs(page)
	event.Details = &searchNotesEventCompletedDetails{
		Query: searchQueryDetails(query),
		Results: searchNotesEventResultsDetails{
			Count:         len(page.Results),
			ReturnedRefs:  returnedRefs,
			ResultSetHash: hashStrings(returnedRefs),
		},
	}

	if err := s.recordEvent(ctx, event); err != nil {
		return fmt.Errorf("record search completed event: %w", err)
	}

	return nil
}

type searchNotesEventFailedDetails struct {
	Query  searchNotesEventQueryDetails `json:"query"`
	Reason string                       `json:"reason"`
}

func (searchNotesEventFailedDetails) EventType() audit.EventType {
	return audit.EventTypeVaultSearchFailed
}

func (s *Service) recordSearchNotesError(
	ctx context.Context,
	principal auth.Principal,
	query domain.SearchNotesQuery,
	err error,
	startedAt time.Time,
) error {
	event := createEvent(ctx, principal, startedAt)
	status, code, reason := classifySearchNotesError(err)

	event.Outcome = s.outcome(startedAt, status, code)
	event.Policy.MappingChecksum = principal.MappingChecksum

	event.Details = &searchNotesEventFailedDetails{
		Query:  searchQueryDetails(query),
		Reason: reason,
	}

	if err := s.recordEvent(ctx, event); err != nil {
		return fmt.Errorf("record search error event: %w", err)
	}

	return nil
}

func classifySearchNotesError(err error) (audit.OutcomeStatus, audit.OutcomeCode, string) {
	switch {
	case errors.Is(err, domain.ErrInvalidSearch):
		return audit.OutcomeStatusFailed, audit.OutcomeCodeInvalidArgument, reasonInvalidSearchQuery
	case errors.Is(err, domain.ErrVaultUnavailable):
		return audit.OutcomeStatusError, audit.OutcomeCodeUnavailable, reasonVaultUnavailable
	default:
		return audit.OutcomeStatusError, audit.OutcomeCodeInternal, reasonRepositoryError
	}
}

func searchQueryDetails(query domain.SearchNotesQuery) searchNotesEventQueryDetails {
	textHash := hashStringsPtr(query.Text.Terms)
	pageTokenHash := hashStringPtr(query.Cursor)

	return searchNotesEventQueryDetails{
		TextHash:            textHash,
		HasText:             len(query.Text.Terms) > 0,
		IncludePathPrefixes: cloneStrings(query.Path.IncludePrefixes),
		ExcludePathPrefixes: cloneStrings(query.Path.ExcludePrefixes),
		AllTags:             stringsFromTags(query.Tags.All),
		AnyTags:             stringsFromTags(query.Tags.Any),
		PageSize:            query.NormalizedLimit(),
		PageTokenHash:       pageTokenHash,
	}
}

func cloneStrings(values []string) []string {
	return append(make([]string, 0, len(values)), values...)
}

func stringsFromTags(tags []domain.Tag) []string {
	tagStrings := make([]string, 0, len(tags))
	for _, tag := range tags {
		normalized := strings.TrimSpace(string(tag))
		if normalized == "" {
			continue
		}

		tagStrings = append(tagStrings, normalized)
	}

	return tagStrings
}

func returnedSearchResultRefs(page domain.SearchNotesPage) []string {
	refs := make([]string, 0, len(page.Results))
	for _, result := range page.Results {
		refs = append(refs, result.Ref.Path.String())
	}

	return refs
}

func matchedSearchResultScopes(page domain.SearchNotesPage, principalScopes []domain.Scope) []domain.Scope {
	seen := map[domain.Scope]bool{}
	matches := make([]domain.Scope, 0, len(principalScopes))

	for _, result := range page.Results {
		for _, scope := range result.Metadata.Access.MatchedScopes(principalScopes) {
			if seen[scope] {
				continue
			}

			seen[scope] = true
			matches = append(matches, scope)
		}
	}

	return matches
}

func hashStringPtr(value string) *string {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return nil
	}

	hashed := hashStrings([]string{normalized})
	return &hashed
}

func hashStringsPtr(values []string) *string {
	if len(values) == 0 {
		return nil
	}

	hashed := hashStrings(values)
	return &hashed
}

func hashStrings(values []string) string {
	hasher := sha256.New()
	for _, value := range values {
		hasher.Write([]byte(value))
		hasher.Write([]byte{0})
	}

	return "sha256:" + hex.EncodeToString(hasher.Sum(nil))
}
