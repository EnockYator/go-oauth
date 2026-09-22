package hash

import (
	"crypto/sha256"
	"encoding/hex"
)

func HashSessionID(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
