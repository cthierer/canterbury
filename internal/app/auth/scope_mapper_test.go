package auth

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/cthierer/canterbury/internal/domain/vault"
)

func TestNewScopeMapper(t *testing.T) {
	source := fakeMappingSource{
		document: MappingDocument{
			Checksum: "sha256:test",
			Subjects: []MappingSubject{
				{
					Issuer:  "https://auth.example.test",
					Subject: "user_123",
					Scopes:  []vault.Scope{"personal-agent"},
				},
			},
		},
	}

	mapper, err := NewScopeMapper(context.Background(), source)
	if err != nil {
		t.Fatalf("NewScopeMapper() error = %v", err)
	}

	got, err := mapper.LookupScopes(context.Background(), "https://auth.example.test", "user_123")
	if err != nil {
		t.Fatalf("LookupScopes() error = %v", err)
	}

	if got.MappingChecksum != "sha256:test" {
		t.Fatalf("MappingChecksum = %q, want sha256:test", got.MappingChecksum)
	}

	wantScopes := []vault.Scope{"personal-agent"}
	if !reflect.DeepEqual(got.Scopes, wantScopes) {
		t.Fatalf("Scopes = %#v, want %#v", got.Scopes, wantScopes)
	}
}

func TestNewScopeMapperRejectsNilSource(t *testing.T) {
	_, err := NewScopeMapper(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error")
	}

	if err.Error() != "mapping source is required" {
		t.Fatalf("NewScopeMapper() error = %v, want mapping source required", err)
	}
}

func TestNewScopeMapperReturnsSourceErrors(t *testing.T) {
	sourceErr := errors.New("read mapping")

	_, err := NewScopeMapper(context.Background(), fakeMappingSource{err: sourceErr})
	if !errors.Is(err, sourceErr) {
		t.Fatalf("NewScopeMapper() error = %v, want %v", err, sourceErr)
	}
}

func TestNewScopeMapperRejectsInvalidSubjects(t *testing.T) {
	tests := []struct {
		name    string
		subject MappingSubject
		want    string
	}{
		{
			name: "blank issuer",
			subject: MappingSubject{
				Issuer:  " ",
				Subject: "user_123",
				Scopes:  []vault.Scope{"personal-agent"},
			},
			want: "issuer must not be blank",
		},
		{
			name: "blank subject",
			subject: MappingSubject{
				Issuer:  "https://auth.example.test",
				Subject: " ",
				Scopes:  []vault.Scope{"personal-agent"},
			},
			want: "subject must not be blank",
		},
		{
			name: "empty scopes",
			subject: MappingSubject{
				Issuer:  "https://auth.example.test",
				Subject: "user_123",
			},
			want: `scope mapping for issuer "https://auth.example.test", subject "user_123" must include at least one scope`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := fakeMappingSource{
				document: MappingDocument{
					Checksum: "sha256:test",
					Subjects: []MappingSubject{test.subject},
				},
			}

			_, err := NewScopeMapper(context.Background(), source)
			if err == nil {
				t.Fatal("expected error")
			}

			if err.Error() != test.want {
				t.Fatalf("NewScopeMapper() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestNewScopeMapperRejectsDuplicateSubjects(t *testing.T) {
	source := fakeMappingSource{
		document: MappingDocument{
			Checksum: "sha256:test",
			Subjects: []MappingSubject{
				{
					Issuer:  "https://auth.example.test",
					Subject: "user_123",
					Scopes:  []vault.Scope{"personal-agent"},
				},
				{
					Issuer:  " https://auth.example.test ",
					Subject: " user_123 ",
					Scopes:  []vault.Scope{"public-site"},
				},
			},
		},
	}

	_, err := NewScopeMapper(context.Background(), source)
	if err == nil {
		t.Fatal("expected error")
	}

	if err.Error() != `duplicate scope mapping for issuer "https://auth.example.test", subject "user_123"` {
		t.Fatalf("NewScopeMapper() error = %v, want duplicate subject", err)
	}
}

func TestScopeMapperLookupScopesReturnsEmptyScopesForUnknownSubject(t *testing.T) {
	mapper := newTestScopeMapper(t)

	got, err := mapper.LookupScopes(context.Background(), "https://auth.example.test", "missing_user")
	if err != nil {
		t.Fatalf("LookupScopes() error = %v", err)
	}

	if got.MappingChecksum != "sha256:test" {
		t.Fatalf("MappingChecksum = %q, want sha256:test", got.MappingChecksum)
	}

	if len(got.Scopes) != 0 {
		t.Fatalf("Scopes = %#v, want empty", got.Scopes)
	}
}

func TestScopeMapperLookupScopesHonorsCanceledContext(t *testing.T) {
	mapper := newTestScopeMapper(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := mapper.LookupScopes(ctx, "https://auth.example.test", "user_123")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("LookupScopes() error = %v, want %v", err, context.Canceled)
	}
}

func TestScopeMapperCopiesScopeSlices(t *testing.T) {
	sourceScopes := []vault.Scope{"personal-agent"}
	source := fakeMappingSource{
		document: MappingDocument{
			Checksum: "sha256:test",
			Subjects: []MappingSubject{
				{
					Issuer:  "https://auth.example.test",
					Subject: "user_123",
					Scopes:  sourceScopes,
				},
			},
		},
	}

	mapper, err := NewScopeMapper(context.Background(), source)
	if err != nil {
		t.Fatalf("NewScopeMapper() error = %v", err)
	}

	sourceScopes[0] = "mutated"

	first, err := mapper.LookupScopes(context.Background(), "https://auth.example.test", "user_123")
	if err != nil {
		t.Fatalf("LookupScopes() first error = %v", err)
	}

	first.Scopes[0] = "mutated-again"

	second, err := mapper.LookupScopes(context.Background(), "https://auth.example.test", "user_123")
	if err != nil {
		t.Fatalf("LookupScopes() second error = %v", err)
	}

	want := []vault.Scope{"personal-agent"}
	if !reflect.DeepEqual(second.Scopes, want) {
		t.Fatalf("Scopes = %#v, want %#v", second.Scopes, want)
	}
}

func newTestScopeMapper(t *testing.T) *ScopeMapper {
	t.Helper()

	mapper, err := NewScopeMapper(context.Background(), fakeMappingSource{
		document: MappingDocument{
			Checksum: "sha256:test",
			Subjects: []MappingSubject{
				{
					Issuer:  "https://auth.example.test",
					Subject: "user_123",
					Scopes:  []vault.Scope{"personal-agent", "public-site"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("NewScopeMapper() error = %v", err)
	}

	return mapper
}

type fakeMappingSource struct {
	document MappingDocument
	err      error
}

func (source fakeMappingSource) LoadMapping(ctx context.Context) (MappingDocument, error) {
	if err := ctx.Err(); err != nil {
		return MappingDocument{}, err
	}

	if source.err != nil {
		return MappingDocument{}, fmt.Errorf("fake load mapping: %w", source.err)
	}

	return source.document, nil
}
