package main

import (
	"crypto/md5"
	"encoding/hex"
)

func hash(text string) string {
	hasher := md5.New()
	hasher.Write([]byte(text))
	s := hex.EncodeToString(hasher.Sum(nil))
	return s[:12]
	// 	return s
}
