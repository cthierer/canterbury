package connectrpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"time"

	"connectrpc.com/connect"
	vaultv1 "github.com/cthierer/canterbury/gen/go/canterbury/vault/v1"
	"github.com/cthierer/canterbury/gen/go/canterbury/vault/v1/vaultv1connect"
	appvault "github.com/cthierer/canterbury/internal/app/vault"
	domainvault "github.com/cthierer/canterbury/internal/domain/vault"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var _ vaultv1connect.VaultServiceHandler = (*VaultServiceHandler)(nil)

type VaultApplication interface {
	ReadNote(ctx context.Context, path domainvault.NotePath) (domainvault.Note, error)
	SearchNotes(ctx context.Context, query domainvault.SearchNotesQuery) (domainvault.SearchNotesPage, error)
}

// VaultServiceHandler is the Connect transport adapter for vault RPCs.
type VaultServiceHandler struct {
	vault VaultApplication
}

// NewVaultServiceHandler creates a Connect vault service handler.
func NewVaultServiceHandler(vault VaultApplication) (*VaultServiceHandler, error) {
	if vault == nil {
		return nil, fmt.Errorf("vault application service is required")
	}

	return &VaultServiceHandler{vault: vault}, nil
}

// ReadNote handles read note requests.
func (h *VaultServiceHandler) ReadNote(
	ctx context.Context,
	req *connect.Request[vaultv1.ReadNoteRequest],
) (*connect.Response[vaultv1.ReadNoteResponse], error) {
	noteRef := req.Msg.Ref
	if noteRef == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("ref is a required parameter"))
	}

	notePath, err := domainvault.NewNotePath(noteRef.Path)
	if err != nil {
		slog.DebugContext(ctx, "could not convert path to NotePath", "err", err)
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("ref must contain a valid path"))
	}

	note, err := h.vault.ReadNote(ctx, notePath)
	if err != nil {
		slog.ErrorContext(ctx, "encountered an error reading vault note", "err", err)
		err = classifyError(req.Msg, err)
		return nil, err
	}

	properties, err := fromMap(note.Metadata.Frontmatter)
	if err != nil {
		slog.ErrorContext(ctx, "encounteed an error formatting file frontmatter", "err", err)
		return nil, connect.NewError(connect.CodeUnknown, errors.New("unexpected error formatting frontmatter"))
	}

	noteMsg := &vaultv1.Note{
		Ref: &vaultv1.NoteRef{
			Path: note.Ref.Path.String(),
		},
		Metadata: &vaultv1.NoteMetadata{
			Title:      note.Metadata.Title,
			Tags:       fromTags(note.Metadata.Tags),
			SizeBytes:  note.Metadata.SizeBytes,
			ModifiedAt: fromTimestamp(note.Metadata.ModifiedAt),
			Properties: properties,
		},
		Content: note.Content,
	}

	readNoteResp := &vaultv1.ReadNoteResponse{
		Note: noteMsg,
	}

	resp := connect.NewResponse(readNoteResp)

	return resp, nil
}

// SearchNotes handles search notes requests.
func (h *VaultServiceHandler) SearchNotes(
	context.Context,
	*connect.Request[vaultv1.SearchNotesRequest],
) (*connect.Response[vaultv1.SearchNotesResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("search notes is not implemented"))
}

func fromTags(tags []domainvault.Tag) []string {
	tagStrings := make([]string, len(tags))
	for i, tagValue := range tags {
		tagStrings[i] = string(tagValue)
	}
	return tagStrings
}

func fromTimestamp(timestamp time.Time) *timestamppb.Timestamp {
	return timestamppb.New(timestamp)
}

func fromMap(data map[string]any) (*structpb.Struct, error) {
	normalized, err := normalizeStructMap(data)
	if err != nil {
		return nil, err
	}

	return structpb.NewStruct(normalized)
}

func normalizeStructMap(data map[string]any) (map[string]any, error) {
	normalized := make(map[string]any, len(data))

	for key, value := range data {
		normalizedValue, err := normalizeStructValue(value)
		if err != nil {
			return nil, fmt.Errorf("normalize property %q: %w", key, err)
		}

		normalized[key] = normalizedValue
	}

	return normalized, nil
}

func normalizeStructValue(value any) (any, error) {
	switch typedValue := value.(type) {
	case nil:
		return nil, nil
	case bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64,
		float32, float64, string, []byte, json.Number:
		return typedValue, nil
	case time.Time:
		return typedValue.Format(time.RFC3339Nano), nil
	case map[string]any:
		return normalizeStructMap(typedValue)
	case []any:
		return normalizeStructList(typedValue)
	}

	return normalizeReflectValue(reflect.ValueOf(value))
}

func normalizeStructList(data []any) ([]any, error) {
	normalized := make([]any, len(data))

	for i, value := range data {
		normalizedValue, err := normalizeStructValue(value)
		if err != nil {
			return nil, fmt.Errorf("normalize list item %d: %w", i, err)
		}

		normalized[i] = normalizedValue
	}

	return normalized, nil
}

func normalizeReflectValue(value reflect.Value) (any, error) {
	if !value.IsValid() {
		return nil, nil
	}

	if value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface {
		if value.IsNil() {
			return nil, nil
		}

		return normalizeStructValue(value.Elem().Interface())
	}

	switch value.Kind() {
	case reflect.Bool:
		return value.Bool(), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return value.Int(), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return value.Uint(), nil
	case reflect.Float32, reflect.Float64:
		return value.Float(), nil
	case reflect.String:
		return value.String(), nil
	case reflect.Map:
		return normalizeReflectMap(value)
	case reflect.Slice, reflect.Array:
		return normalizeReflectList(value)
	default:
		return nil, fmt.Errorf("unsupported value type %T", value.Interface())
	}
}

func normalizeReflectMap(data reflect.Value) (map[string]any, error) {
	if data.Type().Key().Kind() != reflect.String {
		return nil, fmt.Errorf("unsupported map key type %s", data.Type().Key())
	}

	normalized := make(map[string]any, data.Len())
	for _, key := range data.MapKeys() {
		normalizedValue, err := normalizeStructValue(data.MapIndex(key).Interface())
		if err != nil {
			return nil, fmt.Errorf("normalize property %q: %w", key.String(), err)
		}

		normalized[key.String()] = normalizedValue
	}

	return normalized, nil
}

func normalizeReflectList(data reflect.Value) ([]any, error) {
	normalized := make([]any, data.Len())

	for i := 0; i < data.Len(); i++ {
		normalizedValue, err := normalizeStructValue(data.Index(i).Interface())
		if err != nil {
			return nil, fmt.Errorf("normalize list item %d: %w", i, err)
		}

		normalized[i] = normalizedValue
	}

	return normalized, nil
}

func classifyError(req *vaultv1.ReadNoteRequest, err error) error {
	if errors.Is(err, domainvault.ErrNoteNotFound) {
		return connect.NewError(connect.CodeNotFound, fmt.Errorf("requested note %q not found", req.Ref.Path))
	}

	if errors.Is(err, appvault.ErrPermissionDenied) {
		return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("permission denied; check your authorization scopes"))
	}

	if errors.Is(err, domainvault.ErrInvalidNotePath) {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid note path %q", req.Ref.Path))
	}

	if errors.Is(err, domainvault.ErrVaultUnavailable) {
		return connect.NewError(connect.CodeUnavailable, fmt.Errorf("vault cannot be accessed; try again later"))
	}

	if errors.Is(err, context.Canceled) {
		return connect.NewError(connect.CodeCanceled, fmt.Errorf("request canceled"))
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return connect.NewError(connect.CodeDeadlineExceeded, fmt.Errorf("deadline exceeded"))
	}

	return connect.NewError(connect.CodeUnknown, fmt.Errorf("an unknown error occurred"))
}
