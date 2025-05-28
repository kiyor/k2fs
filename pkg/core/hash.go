package core

import (
	"crypto/md5"
	"encoding/hex"
)

// Hash generates a short MD5 hash of a string.
// It returns the first 12 characters of the hex-encoded MD5 hash.
func Hash(text string) string {
	hasher := md5.New()
	hasher.Write([]byte(text))
	s := hex.EncodeToString(hasher.Sum(nil))
	return s[:12]
}
