package shortener

import (
	"math/rand"
)

const (
	Charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	Length  = 6
)

func Generate() string {
	b := make([]byte, Length)
	for i := range b {
		b[i] = Charset[rand.Intn(len(Charset))]
	}
	return string(b)
}
