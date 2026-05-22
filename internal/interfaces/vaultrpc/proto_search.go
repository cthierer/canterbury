package connectrpc

import (
	"fmt"

	vaultv1 "github.com/cthierer/canterbury/gen/go/canterbury/vault/v1"
	domainvault "github.com/cthierer/canterbury/internal/domain/vault"
)

func searchResultsToProto(results []domainvault.SearchNoteResult) ([]*vaultv1.SearchNoteResult, error) {
	resultsMsg := make([]*vaultv1.SearchNoteResult, len(results))

	for i, result := range results {
		ref := noteRefToProto(result.Ref)

		metadata, err := noteMetadataToProto(result.Metadata)
		if err != nil {
			return nil, fmt.Errorf("build metadata proto: %w", err)
		}

		resultMsg := &vaultv1.SearchNoteResult{
			Ref:      ref,
			Metadata: metadata,
			Snippet:  result.Snippet,
		}

		resultsMsg[i] = resultMsg
	}

	return resultsMsg, nil
}
