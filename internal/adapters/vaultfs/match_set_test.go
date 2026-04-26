package vaultfs

import "testing"

func TestContainsAll(t *testing.T) {
	searchSet := map[string]struct{}{
		"a": {},
		"b": {},
	}

	candidateSet := map[string]struct{}{
		"a": {},
		"b": {},
		"c": {},
	}

	if !containsAll(searchSet, candidateSet) {
		t.Fatal("expected set to contain all values")
	}
}

func TestContainsAny(t *testing.T) {
	searchSet := map[string]struct{}{
		"a": {},
		"b": {},
	}

	candidateSet := map[string]struct{}{
		"c": {},
		"b": {},
	}

	if !containsAny(searchSet, candidateSet) {
		t.Fatal("expected set to contain any value")
	}
}

func TestNormalizeSet(t *testing.T) {
	got := normalizeSet([]string{" A ", "", "a"}, func(value string) string {
		if value == "" {
			return ""
		}

		return normalizeStrings([]string{value})[0]
	})

	if len(got) != 1 {
		t.Fatalf("got %d values, want 1", len(got))
	}

	if _, found := got["a"]; !found {
		t.Fatal("expected normalized value")
	}
}
