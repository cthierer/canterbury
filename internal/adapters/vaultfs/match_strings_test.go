package vaultfs

import (
	"reflect"
	"testing"
)

func TestNormalizeStrings(t *testing.T) {
	got := normalizeStrings([]string{" Canterbury ", "", "VAULT", " \t "})
	want := []string{"canterbury", "vault"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
