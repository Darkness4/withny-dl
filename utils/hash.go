package utils

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
)

// Hash returns the hash of the object.
func Hash(o any) string {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(o); err != nil {
		panic(err)
	}
	hash := sha256.New()
	hash.Write(buf.Bytes())

	hashString := hex.EncodeToString(hash.Sum(nil))
	return hashString
}
