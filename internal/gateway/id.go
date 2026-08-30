package gateway

import (
	"crypto/rand"
	"encoding/hex"
)

// GenerateRequestID generates a unique, bounded request identifier.
// Format: req_<32 hex characters>
func GenerateRequestID() string {
	var b [16]byte
	_, err := rand.Read(b[:])
	if err != nil {
		// Fallback to static zeroed bytes representation if rand fails
		return "req_00000000000000000000000000000000"
	}
	return "req_" + hex.EncodeToString(b[:])
}
