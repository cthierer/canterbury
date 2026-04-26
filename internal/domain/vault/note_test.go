package vault_test

import (
	"testing"

	"github.com/cthierer/canterbury/internal/domain/vault"
)

func TestNewNotePath(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    vault.NotePath
		wantErr bool
	}{
		{
			name:  "accepts relative markdown path",
			value: "Projects/Canterbury.md",
			want:  "Projects/Canterbury.md",
		},
		{
			name:  "cleans relative path",
			value: "Projects/./Canterbury.md",
			want:  "Projects/Canterbury.md",
		},
		{
			name:    "rejects empty path",
			value:   "",
			wantErr: true,
		},
		{
			name:    "rejects absolute path",
			value:   "/Projects/Canterbury.md",
			wantErr: true,
		},
		{
			name:    "rejects traversal",
			value:   "../Canterbury.md",
			wantErr: true,
		},
		{
			name:    "rejects backslashes",
			value:   "Projects\\Canterbury.md",
			wantErr: true,
		},
		{
			name:    "rejects non-markdown path",
			value:   "Projects/Canterbury.txt",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := vault.NewNotePath(test.value)
			if test.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}
