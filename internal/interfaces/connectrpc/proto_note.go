package connectrpc

import (
	"fmt"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	vaultv1 "github.com/cthierer/canterbury/gen/go/canterbury/vault/v1"
	domainvault "github.com/cthierer/canterbury/internal/domain/vault"
)

func noteToProto(note domainvault.Note) (*vaultv1.Note, error) {
	ref := noteRefToProto(note.Ref)

	metadata, err := noteMetadataToProto(note.Metadata)
	if err != nil {
		return nil, fmt.Errorf("build metadata proto: %w", err)
	}

	noteMsg := &vaultv1.Note{
		Ref:      ref,
		Metadata: metadata,
		Content:  note.Content,
	}

	return noteMsg, nil
}

func noteRefToProto(ref domainvault.NoteRef) *vaultv1.NoteRef {
	return &vaultv1.NoteRef{
		Path: ref.Path.String(),
	}
}

func noteMetadataToProto(metadata domainvault.NoteMetadata) (*vaultv1.NoteMetadata, error) {
	propertiesProto, err := frontmatterToProto(metadata.Frontmatter)
	if err != nil {
		return nil, fmt.Errorf("convert frontmatter to properties: %w", err)
	}

	return &vaultv1.NoteMetadata{
		Title:      metadata.Title,
		Tags:       noteTagsToProto(metadata.Tags),
		SizeBytes:  metadata.SizeBytes,
		ModifiedAt: timestampToProto(metadata.ModifiedAt),
		Properties: propertiesProto,
	}, nil
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
