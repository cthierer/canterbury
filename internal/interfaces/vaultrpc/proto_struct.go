package connectrpc

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"google.golang.org/protobuf/types/known/structpb"
)

func frontmatterToProto(frontmatter map[string]any) (*structpb.Struct, error) {
	normalized, err := normalizeStructMap(frontmatter)
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
