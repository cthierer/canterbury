package vaultrpc

import (
	"strings"
	"testing"
	"time"
)

func TestFrontmatterToProtoNormalizesYAMLValues(t *testing.T) {
	timestamp := time.Date(2026, 4, 29, 14, 30, 15, 123, time.UTC)

	properties, err := frontmatterToProto(map[string]any{
		"created": timestamp,
		"aliases": []string{
			"alpha",
			"beta",
		},
		"nested": map[string]any{
			"due": timestamp,
			"counts": []int{
				1,
				2,
			},
		},
		"typed_map": map[string]string{
			"status": "ready",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := properties.AsMap()
	wantTimestamp := timestamp.Format(time.RFC3339Nano)
	if got["created"] != wantTimestamp {
		t.Fatalf("got created %#v, want %q", got["created"], wantTimestamp)
	}

	aliases, ok := got["aliases"].([]any)
	if !ok {
		t.Fatalf("got aliases type %T, want []any", got["aliases"])
	}
	if len(aliases) != 2 || aliases[0] != "alpha" || aliases[1] != "beta" {
		t.Fatalf("got aliases %#v, want alpha and beta", aliases)
	}

	nested, ok := got["nested"].(map[string]any)
	if !ok {
		t.Fatalf("got nested type %T, want map[string]any", got["nested"])
	}
	if nested["due"] != wantTimestamp {
		t.Fatalf("got nested due %#v, want %q", nested["due"], wantTimestamp)
	}

	counts, ok := nested["counts"].([]any)
	if !ok {
		t.Fatalf("got counts type %T, want []any", nested["counts"])
	}
	if len(counts) != 2 || counts[0] != float64(1) || counts[1] != float64(2) {
		t.Fatalf("got counts %#v, want 1 and 2", counts)
	}

	typedMap, ok := got["typed_map"].(map[string]any)
	if !ok {
		t.Fatalf("got typed map type %T, want map[string]any", got["typed_map"])
	}
	if typedMap["status"] != "ready" {
		t.Fatalf("got typed map status %#v, want ready", typedMap["status"])
	}
}

func TestFrontmatterToProtoRejectsNonStringMapKeys(t *testing.T) {
	_, err := frontmatterToProto(map[string]any{
		"bad": map[int]string{
			1: "one",
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "unsupported map key type int") {
		t.Fatalf("got error %q, want unsupported map key type", err)
	}
}
