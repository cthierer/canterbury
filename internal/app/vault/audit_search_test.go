package vault

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/cthierer/canterbury/internal/domain/audit"
	domain "github.com/cthierer/canterbury/internal/domain/vault"
)

func TestClassifySearchNotesError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus audit.OutcomeStatus
		wantCode   audit.OutcomeCode
		wantReason string
	}{
		{
			name:       "invalid search query",
			err:        fmt.Errorf("search notes: %w", domain.ErrInvalidSearch),
			wantStatus: audit.OutcomeStatusFailed,
			wantCode:   audit.OutcomeCodeInvalidArgument,
			wantReason: reasonInvalidSearchQuery,
		},
		{
			name:       "vault unavailable",
			err:        fmt.Errorf("search notes: %w", domain.ErrVaultUnavailable),
			wantStatus: audit.OutcomeStatusError,
			wantCode:   audit.OutcomeCodeUnavailable,
			wantReason: reasonVaultUnavailable,
		},
		{
			name:       "unexpected repository error",
			err:        errors.New("disk vanished"),
			wantStatus: audit.OutcomeStatusError,
			wantCode:   audit.OutcomeCodeInternal,
			wantReason: reasonRepositoryError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotStatus, gotCode, gotReason := classifySearchNotesError(test.err)

			if gotStatus != test.wantStatus {
				t.Fatalf("status = %q, want %q", gotStatus, test.wantStatus)
			}

			if gotCode != test.wantCode {
				t.Fatalf("code = %q, want %q", gotCode, test.wantCode)
			}

			if gotReason != test.wantReason {
				t.Fatalf("reason = %q, want %q", gotReason, test.wantReason)
			}
		})
	}
}

func TestSearchQueryDetails(t *testing.T) {
	details := searchQueryDetails(domain.SearchNotesQuery{
		Text: domain.TextSearch{
			Terms: []string{"canterbury", "audit"},
		},
		Path: domain.PathFilter{
			IncludePrefixes: []string{"Projects/"},
			ExcludePrefixes: []string{"Archive/"},
		},
		Tags: domain.TagFilter{
			All: []domain.Tag{"canterbury"},
			Any: []domain.Tag{"audit"},
		},
		Limit:  10,
		Cursor: "10",
	})

	if details.TextHash == nil {
		t.Fatal("expected text hash")
	}

	if strings.Contains(*details.TextHash, "canterbury") {
		t.Fatalf("text hash exposes raw query term: %q", *details.TextHash)
	}

	if !details.HasText {
		t.Fatal("expected has text")
	}

	if details.PageSize != 10 {
		t.Fatalf("page size = %d, want 10", details.PageSize)
	}

	if details.PageTokenHash == nil {
		t.Fatal("expected page token hash")
	}
}
