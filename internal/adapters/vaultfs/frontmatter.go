package vaultfs

import (
	"errors"
	"fmt"
	"strings"

	"github.com/cthierer/canterbury/internal/domain/vault"
	"gopkg.in/yaml.v3"
)

var errUnclosedFrontmatter = errors.New("unclosed frontmatter block")

type noteDocument struct {
	Body           string
	HasFrontmatter bool
	Frontmatter    map[string]any
	Access         vault.ResourceAccess
	Tags           []vault.Tag
}

func parseNoteDocument(content string) (noteDocument, error) {
	frontmatter, body, hasFrontmatter, err := splitFrontmatter(content)
	if err != nil {
		return noteDocument{}, fmt.Errorf("split frontmatter: %w", err)
	}

	if !hasFrontmatter {
		return noteDocument{
			Body:           body,
			HasFrontmatter: false,
		}, nil
	}

	properties, parsedFrontmatter, err := parseNoteFrontmatter(frontmatter)
	if err != nil {
		return noteDocument{}, fmt.Errorf("parse note frontmatter: %w", err)
	}

	access, err := buildAccess(properties.Access.Scopes)
	if err != nil {
		return noteDocument{}, fmt.Errorf("parse access policy: %w", err)
	}

	tags := buildTags(properties.Tags)

	return noteDocument{
		Body:           body,
		HasFrontmatter: hasFrontmatter,
		Frontmatter:    parsedFrontmatter,
		Access:         access,
		Tags:           tags,
	}, nil
}

func splitFrontmatter(content string) (frontmatter string, body string, has bool, err error) {
	lines := strings.SplitAfter(content, "\n")
	if len(lines) == 0 || !isFrontmatterFence(lines[0]) {
		return "", content, false, nil
	}

	frontmatterStart := len(lines[0])
	frontmatterEnd := frontmatterStart

	for _, line := range lines[1:] {
		if isFrontmatterFence(line) {
			bodyStart := frontmatterEnd + len(line)
			return content[frontmatterStart:frontmatterEnd], content[bodyStart:], true, nil
		}

		frontmatterEnd += len(line)
	}

	return "", "", false, errUnclosedFrontmatter
}

func isFrontmatterFence(line string) bool {
	line = strings.TrimRight(line, " \t\r\n")
	return line == "---"
}

type noteFrontmatter struct {
	Access struct {
		Scopes []string `yaml:"scopes"`
	} `yaml:"access"`
	Tags []string `yaml:"tags"`
}

func buildAccess(scopeStrings []string) (vault.ResourceAccess, error) {
	scopes := make([]vault.Scope, 0, len(scopeStrings))

	for _, scope := range scopeStrings {
		trimmed := strings.TrimSpace(scope)
		if trimmed == "" {
			continue
		}

		parsedScope, err := vault.NewScope(trimmed)
		if err != nil {
			return vault.ResourceAccess{}, fmt.Errorf("parse scope %q: %w", scope, err)
		}

		scopes = append(scopes, parsedScope)
	}

	return vault.ResourceAccess{Scopes: scopes}, nil
}

func buildTags(tagStrings []string) []vault.Tag {
	tags := make([]vault.Tag, 0, len(tagStrings))

	for _, value := range tagStrings {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}

		tag, _ := vault.NewTag(trimmed)
		tags = append(tags, tag)
	}

	return tags
}

func parseNoteFrontmatter(frontmatter string) (properties noteFrontmatter, raw map[string]any, err error) {
	err = yaml.Unmarshal([]byte(frontmatter), &properties)
	if err != nil {
		err = fmt.Errorf("unmarshal frontmatter into struct: %w", err)
		return
	}

	err = yaml.Unmarshal([]byte(frontmatter), &raw)
	if err != nil {
		err = fmt.Errorf("unmarshal frontmatter into map: %w", err)
	}

	return
}
