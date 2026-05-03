package auditlog

import (
	"crypto/rand"
	"fmt"
	"time"

	domain "github.com/cthierer/canterbury/internal/domain/audit"
)

const (
	ulidEncoding           = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	maxULIDTimestampMillis = 1<<48 - 1
)

// IDGenerator creates audit event identifiers from event timestamps.
type IDGenerator interface {
	NewEventID(timestamp time.Time) (domain.EventID, error)
}

type ulidGenerator struct{}

func (ulidGenerator) NewEventID(timestamp time.Time) (domain.EventID, error) {
	ms := timestamp.UnixMilli()
	if ms < 0 || ms > maxULIDTimestampMillis {
		return "", ErrInvalidTimestamp
	}

	ulid, err := makeULID(timestamp)
	if err != nil {
		return "", fmt.Errorf("make ulid for event: %w", err)
	}

	eventID, err := domain.NewEventID(ulid)
	if err != nil {
		return "", fmt.Errorf("make event ID from ulid: %w", err)
	}

	return eventID, nil
}

func makeULID(timestamp time.Time) (string, error) {
	var entropy [10]byte

	if _, err := rand.Read(entropy[:]); err != nil {
		return "", fmt.Errorf("read random ULID entropy: %w", err)
	}

	value := encodeULID(timestamp, entropy)
	return value, nil
}

func encodeULID(timestamp time.Time, entropy [10]byte) string {
	ms := uint64(timestamp.UnixMilli())

	var out [26]byte

	// 48-bit timestamp, encoded into 10 Crockford Base32 chars.
	out[0] = ulidEncoding[(ms>>45)&0x1f]
	out[1] = ulidEncoding[(ms>>40)&0x1f]
	out[2] = ulidEncoding[(ms>>35)&0x1f]
	out[3] = ulidEncoding[(ms>>30)&0x1f]
	out[4] = ulidEncoding[(ms>>25)&0x1f]
	out[5] = ulidEncoding[(ms>>20)&0x1f]
	out[6] = ulidEncoding[(ms>>15)&0x1f]
	out[7] = ulidEncoding[(ms>>10)&0x1f]
	out[8] = ulidEncoding[(ms>>5)&0x1f]
	out[9] = ulidEncoding[ms&0x1f]

	// 80 bits of entropy, encoded into 16 Crockford Base32 chars.
	out[10] = ulidEncoding[(entropy[0]&0xf8)>>3]
	out[11] = ulidEncoding[((entropy[0]&0x07)<<2)|((entropy[1]&0xc0)>>6)]
	out[12] = ulidEncoding[(entropy[1]&0x3e)>>1]
	out[13] = ulidEncoding[((entropy[1]&0x01)<<4)|((entropy[2]&0xf0)>>4)]
	out[14] = ulidEncoding[((entropy[2]&0x0f)<<1)|((entropy[3]&0x80)>>7)]
	out[15] = ulidEncoding[(entropy[3]&0x7c)>>2]
	out[16] = ulidEncoding[((entropy[3]&0x03)<<3)|((entropy[4]&0xe0)>>5)]
	out[17] = ulidEncoding[entropy[4]&0x1f]
	out[18] = ulidEncoding[(entropy[5]&0xf8)>>3]
	out[19] = ulidEncoding[((entropy[5]&0x07)<<2)|((entropy[6]&0xc0)>>6)]
	out[20] = ulidEncoding[(entropy[6]&0x3e)>>1]
	out[21] = ulidEncoding[((entropy[6]&0x01)<<4)|((entropy[7]&0xf0)>>4)]
	out[22] = ulidEncoding[((entropy[7]&0x0f)<<1)|((entropy[8]&0x80)>>7)]
	out[23] = ulidEncoding[(entropy[8]&0x7c)>>2]
	out[24] = ulidEncoding[((entropy[8]&0x03)<<3)|((entropy[9]&0xe0)>>5)]
	out[25] = ulidEncoding[entropy[9]&0x1f]

	return string(out[:])
}
