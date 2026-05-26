package authfs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/cthierer/canterbury/internal/app/auth"
	"github.com/cthierer/canterbury/internal/domain/vault"
)

// LoadMapping reads and parses the configured TOML scope mapping file.
func (loader *Loader) LoadMapping(ctx context.Context) (auth.MappingDocument, error) {
	if err := ctx.Err(); err != nil {
		return auth.MappingDocument{}, err
	}

	fileBytes, err := os.ReadFile(loader.filePath)
	if err != nil {
		return auth.MappingDocument{}, fmt.Errorf("read file %q: %w", loader.filePath, err)
	}

	var file mappingFile
	metadata, err := toml.Decode(string(fileBytes), &file)
	if err != nil {
		return auth.MappingDocument{}, fmt.Errorf("parse auth mapping TOML: %w", err)
	}

	undecoded := metadata.Undecoded()
	if len(undecoded) > 0 {
		return auth.MappingDocument{}, fmt.Errorf("found unrecognized keys %v: %w", undecoded, ErrUnrecognizedKeys)
	}

	mappings := auth.MappingDocument{
		Checksum: computeChecksum(fileBytes),
	}

	for _, entry := range file.Subjects {
		scopes, err := toScopes(entry.Scopes)
		if err != nil {
			return auth.MappingDocument{}, fmt.Errorf("build scopes: %w", err)
		}

		subject := auth.MappingSubject{
			Issuer:  entry.Issuer,
			Subject: entry.Subject,
			Scopes:  scopes,
		}

		mappings.Subjects = append(mappings.Subjects, subject)
	}

	return mappings, nil
}

type mappingFile struct {
	Subjects []subjectEntry `toml:"subjects"`
}

type subjectEntry struct {
	Issuer  string   `toml:"issuer"`
	Subject string   `toml:"subject"`
	Scopes  []string `toml:"scopes"`
}

func toScopes(values []string) ([]vault.Scope, error) {
	scopes := make([]vault.Scope, len(values))
	for i, value := range values {
		var err error
		scopes[i], err = vault.NewScope(value)
		if err != nil {
			return nil, fmt.Errorf("parse document scope: %w", err)
		}
	}

	return scopes, nil
}

func computeChecksum(bytes []byte) string {
	sum := sha256.Sum256(bytes)
	return fmt.Sprintf("sha256:%s", hex.EncodeToString(sum[:]))
}
