package connectrpc

import (
	"fmt"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	vaultv1 "github.com/cthierer/canterbury/gen/go/canterbury/vault/v1"
	domainvault "github.com/cthierer/canterbury/internal/domain/vault"
)

func noteToProto(note domainvault.Note) (*vaultv1.Note, error) {
	propertiesProto, err := frontmatterToProto(note.Metadata.Frontmatter)
	if err != nil {
		return nil, fmt.Errorf("convert frontmatter to properties: %w", err)
	}

	noteMsg := &vaultv1.Note{
		Ref: &vaultv1.NoteRef{
			Path: note.Ref.Path.String(),
		},
		Metadata: &vaultv1.NoteMetadata{
			Title:      note.Metadata.Title,
			Tags:       noteTagsToProto(note.Metadata.Tags),
			SizeBytes:  note.Metadata.SizeBytes,
			ModifiedAt: timestampToProto(note.Metadata.ModifiedAt),
			Properties: propertiesProto,
		},
		Content: note.Content,
	}

	return noteMsg, nil
}

func noteTagsToProto(tags []domainvault.Tag) []string {
	tagStrings := make([]string, len(tags))
	for i, tagValue := range tags {
		tagStrings[i] = string(tagValue)
	}
	return tagStrings
}

func timestampToProto(timestamp time.Time) *timestamppb.Timestamp {
	return timestamppb.New(timestamp)
}
