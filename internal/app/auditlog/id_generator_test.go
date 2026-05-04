package auditlog

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/cthierer/canterbury/internal/app/idgen"
)

func TestULIDGeneratorNewEventID(t *testing.T) {
	generator := ulidGenerator{}

	t.Run("creates valid event ID", func(t *testing.T) {
		id, err := generator.NewEventID(time.UnixMilli(1))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got := id.String()
		if len(got) != 26 {
			t.Fatalf("got length %d, want 26", len(got))
		}

		for _, char := range got {
			if !strings.ContainsRune("0123456789ABCDEFGHJKMNPQRSTVWXYZ", char) {
				t.Fatalf("got invalid ULID character %q in %q", char, got)
			}
		}
	})

	t.Run("rejects timestamp before Unix epoch", func(t *testing.T) {
		_, err := generator.NewEventID(time.UnixMilli(-1))

		if !errors.Is(err, idgen.ErrInvalidTimestamp) {
			t.Fatalf("got error %v, want %v", err, idgen.ErrInvalidTimestamp)
		}
	})

	t.Run("rejects timestamp beyond ULID range", func(t *testing.T) {
		_, err := generator.NewEventID(time.UnixMilli(1 << 48))

		if !errors.Is(err, idgen.ErrInvalidTimestamp) {
			t.Fatalf("got error %v, want %v", err, idgen.ErrInvalidTimestamp)
		}
	})
}
