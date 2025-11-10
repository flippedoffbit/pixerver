package uuidv7

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// New returns a small, RFC-like UUIDv7 string.
// It uses the current Unix epoch milliseconds as the high 48 bits
// and fills the remaining bits with cryptographic random data.
// The layout sets the version to 7 and the RFC 4122 variant bits.
func New() string {
	var u [16]byte
	// timestamp in milliseconds (48 bits)
	now := uint64(time.Now().UnixMilli())
	u[0] = byte(now >> 40)
	u[1] = byte(now >> 32)
	u[2] = byte(now >> 24)
	u[3] = byte(now >> 16)
	u[4] = byte(now >> 8)
	u[5] = byte(now)

	// fill remaining bytes with crypto-random
	if _, err := rand.Read(u[6:]); err != nil {
		// rand.Read should not fail in normal environments; if it does,
		// fall back to mixing in lower-resolution time bytes.
		t := uint64(time.Now().UnixNano())
		for i := 6; i < 16; i++ {
			u[i] = byte(t >> (uint((i-6)%8) * 8))
		}
	}

	// set version = 7 (four most significant bits of u[6])
	u[6] = (u[6] & 0x0f) | 0x70

	// set variant = RFC 4122 (two most significant bits of u[8] to 10)
	u[8] = (u[8] & 0x3f) | 0x80

	s := hex.EncodeToString(u[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s", s[0:8], s[8:12], s[12:16], s[16:20], s[20:32])
}
