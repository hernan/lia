package shortener

import (
	"crypto/rand"
)

const (
	Charset = "abcdefghjkmnpqrstuvwxyz023456789"
	Length  = 6
)

func Generate() string {
	b := make([]byte, Length)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	for i := range b {
		b[i] = Charset[b[i]%32]
	}
	return string(b)
}
