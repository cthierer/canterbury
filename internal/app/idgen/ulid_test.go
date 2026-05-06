package idgen

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNewULID(t *testing.T) {
	t.Run("creates valid ULID", func(t *testing.T) {
		got, err := NewULID(time.UnixMilli(1))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(got) != 26 {
			t.Fatalf("got length %d, want 26", len(got))
		}

		for _, char := range got {
			if !strings.ContainsRune(ulidEncoding, char) {
				t.Fatalf("got invalid ULID character %q in %q", char, got)
			}
		}
	})

	t.Run("encodes timestamp prefix", func(t *testing.T) {
		got, err := NewULID(time.UnixMilli(1))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.HasPrefix(got, "0000000001") {
			t.Fatalf("got %q, want timestamp prefix", got)
		}
	})

	t.Run("rejects timestamp before Unix epoch", func(t *testing.T) {
		_, err := NewULID(time.UnixMilli(-1))

		if !errors.Is(err, ErrInvalidTimestamp) {
			t.Fatalf("got error %v, want %v", err, ErrInvalidTimestamp)
		}
	})

	t.Run("rejects timestamp beyond ULID range", func(t *testing.T) {
		_, err := NewULID(time.UnixMilli(maxULIDTimestampMillis + 1))

		if !errors.Is(err, ErrInvalidTimestamp) {
			t.Fatalf("got error %v, want %v", err, ErrInvalidTimestamp)
		}
	})
}

func TestEncodeULID(t *testing.T) {
	t.Run("encodes zero timestamp and entropy", func(t *testing.T) {
		got := encodeULID(time.UnixMilli(0), [10]byte{})

		if got != "00000000000000000000000000" {
			t.Fatalf("got %q, want zero ULID", got)
		}
	})

	t.Run("encodes timestamp before entropy", func(t *testing.T) {
		got := encodeULID(time.UnixMilli(1), [10]byte{})

		if got != "00000000010000000000000000" {
			t.Fatalf("got %q, want timestamp-first ULID", got)
		}
	})

	t.Run("encodes max timestamp and entropy", func(t *testing.T) {
		entropy := [10]byte{}
		for i := range entropy {
			entropy[i] = 0xff
		}

		got := encodeULID(time.UnixMilli(maxULIDTimestampMillis), entropy)

		if got != "7ZZZZZZZZZZZZZZZZZZZZZZZZZ" {
			t.Fatalf("got %q, want max ULID", got)
		}
	})

	t.Run("encodes entropy across byte boundaries", func(t *testing.T) {
		entropy := [10]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
		timestamp := time.Date(2026, time.May, 3, 18, 25, 43, 123000000, time.UTC)

		got := encodeULID(timestamp, entropy)

		if got != "01KQQHDM6K000G40R40M30E209" {
			t.Fatalf("got %q, want fixed ULID", got)
		}
	})
}

func TestEncodeULIDSortsByTimestamp(t *testing.T) {
	entropy := [10]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	earlier := encodeULID(time.UnixMilli(1000), entropy)
	later := encodeULID(time.UnixMilli(1001), entropy)

	if earlier >= later {
		t.Fatalf("earlier ULID %q should sort before later ULID %q", earlier, later)
	}
}
