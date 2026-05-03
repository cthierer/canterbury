package vault

import (
	"errors"
	"fmt"
	"testing"

	"github.com/cthierer/canterbury/internal/domain/audit"
	domain "github.com/cthierer/canterbury/internal/domain/vault"
)

func TestClassifyReadNoteError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus audit.OutcomeStatus
		wantCode   audit.OutcomeCode
		wantReason string
	}{
		{
			name:       "invalid note path",
			err:        fmt.Errorf("read note: %w", domain.ErrInvalidNotePath),
			wantStatus: audit.OutcomeStatusFailed,
			wantCode:   audit.OutcomeCodeInvalidArgument,
			wantReason: reasonInvalidNotePath,
		},
		{
			name:       "note not found",
			err:        fmt.Errorf("read note: %w", domain.ErrNoteNotFound),
			wantStatus: audit.OutcomeStatusFailed,
			wantCode:   audit.OutcomeCodeNotFound,
			wantReason: reasonNoteNotFound,
		},
		{
			name:       "vault unavailable",
			err:        fmt.Errorf("read note: %w", domain.ErrVaultUnavailable),
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
			gotStatus, gotCode, gotReason := classifyReadNoteError(test.err)

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
