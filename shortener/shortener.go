package shortener

import (
	"crypto/rand"
	"math/big"
)

const (
	Charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	Length  = 6
)

func Generate() string {
	b := make([]byte, Length)
	charsetLen := big.NewInt(int64(len(Charset)))
	for i := range b {
		n, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			panic("crypto/rand failed: " + err.Error())
		}
		b[i] = Charset[n.Int64()]
	}
	return string(b)
}
