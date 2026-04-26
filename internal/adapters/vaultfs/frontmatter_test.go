package vaultfs

import (
	"errors"
	"reflect"
	"testing"

	"github.com/cthierer/canterbury/internal/domain/vault"
)

func TestSplitFrontmatter(t *testing.T) {
	tests := []struct {
		name            string
		content         string
		wantFrontmatter string
		wantBody        string
		wantHas         bool
		wantErr         error
	}{
		{
			name:     "returns full content when frontmatter is absent",
			content:  "# Hello\n\nBody\n",
			wantBody: "# Hello\n\nBody\n",
		},
		{
			name: "splits frontmatter and body",
			content: "---\n" +
				"access:\n" +
				"  scopes:\n" +
				"    - personal-agent\n" +
				"---\n" +
				"# Hello\n",
			wantFrontmatter: "access:\n  scopes:\n    - personal-agent\n",
			wantBody:        "# Hello\n",
			wantHas:         true,
		},
		{
			name: "supports crlf line endings",
			content: "---\r\n" +
				"tags:\r\n" +
				"  - project\r\n" +
				"---\r\n" +
				"Body\r\n",
			wantFrontmatter: "tags:\r\n  - project\r\n",
			wantBody:        "Body\r\n",
			wantHas:         true,
		},
		{
			name:            "supports empty frontmatter",
			content:         "---\n---\nBody\n",
			wantFrontmatter: "",
			wantBody:        "Body\n",
			wantHas:         true,
		},
		{
			name:    "requires closing fence",
			content: "---\naccess:\n",
			wantErr: errUnclosedFrontmatter,
		},
		{
			name:     "ignores non-leading fence",
			content:  "\n---\naccess:\n---\nBody\n",
			wantBody: "\n---\naccess:\n---\nBody\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotFrontmatter, gotBody, gotHas, err := splitFrontmatter(test.content)
			if test.wantErr != nil {
				if !errors.Is(err, test.wantErr) {
					t.Fatalf("got error %v, want %v", err, test.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if gotFrontmatter != test.wantFrontmatter {
				t.Fatalf("got frontmatter %q, want %q", gotFrontmatter, test.wantFrontmatter)
			}

			if gotBody != test.wantBody {
				t.Fatalf("got body %q, want %q", gotBody, test.wantBody)
			}

			if gotHas != test.wantHas {
				t.Fatalf("got has %t, want %t", gotHas, test.wantHas)
			}
		})
	}
}

func TestParseNoteDocument(t *testing.T) {
	content := "---\n" +
		"access:\n" +
		"  scopes:\n" +
		"    - personal-agent\n" +
		"    - ' '\n" +
		"    - public-site\n" +
		"tags:\n" +
		"  - project\n" +
		"  - ' '\n" +
		"  - Canterbury\n" +
		"title: Example\n" +
		"---\n" +
		"# Example\n"

	document, err := parseNoteDocument(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if document.Body != "# Example\n" {
		t.Fatalf("got body %q, want %q", document.Body, "# Example\n")
	}

	if !document.HasFrontmatter {
		t.Fatal("expected frontmatter")
	}

	wantScopes := []vault.Scope{"personal-agent", "public-site"}
	if !reflect.DeepEqual(document.Access.Scopes, wantScopes) {
		t.Fatalf("got scopes %#v, want %#v", document.Access.Scopes, wantScopes)
	}

	wantTags := []vault.Tag{"project", "Canterbury"}
	if !reflect.DeepEqual(document.Tags, wantTags) {
		t.Fatalf("got tags %#v, want %#v", document.Tags, wantTags)
	}

	if got := document.Frontmatter["title"]; got != "Example" {
		t.Fatalf("got raw title %#v, want %q", got, "Example")
	}
}

func TestParseNoteDocumentWithoutFrontmatter(t *testing.T) {
	content := "# Hello\n"

	document, err := parseNoteDocument(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if document.Body != content {
		t.Fatalf("got body %q, want %q", document.Body, content)
	}

	if document.HasFrontmatter {
		t.Fatal("did not expect frontmatter")
	}

	if document.Frontmatter != nil {
		t.Fatalf("got frontmatter %#v, want nil", document.Frontmatter)
	}
}

func TestParseNoteDocumentRejectsInvalidFrontmatter(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name: "invalid yaml",
			content: "---\n" +
				"access: [\n" +
				"---\n" +
				"Body\n",
		},
		{
			name: "invalid access scopes shape",
			content: "---\n" +
				"access:\n" +
				"  scopes: personal-agent\n" +
				"---\n" +
				"Body\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseNoteDocument(test.content)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestBuildAccessSkipsBlankScopes(t *testing.T) {
	access, err := buildAccess([]string{" personal-agent ", " ", "public-site"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []vault.Scope{"personal-agent", "public-site"}
	if !reflect.DeepEqual(access.Scopes, want) {
		t.Fatalf("got scopes %#v, want %#v", access.Scopes, want)
	}
}

func TestBuildTagsTrimsAndSkipsBlankTags(t *testing.T) {
	got := buildTags([]string{" project ", " ", "Canterbury"})
	want := []vault.Tag{"project", "Canterbury"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got tags %#v, want %#v", got, want)
	}
}
