package shortener

import (
	"testing"
)

func TestGenerateLength(t *testing.T) {
	for i := 0; i < 100; i++ {
		code := Generate()
		if len(code) != Length {
			t.Errorf("expected length %d, got %d for code %q", Length, len(code), code)
		}
	}
}

func TestGenerateCharset(t *testing.T) {
	for i := 0; i < 100; i++ {
		code := Generate()
		for _, c := range code {
			if !contains(Charset, c) {
				t.Errorf("character %q not in charset in code %q", c, code)
			}
		}
	}
}

func contains(s string, r rune) bool {
	for _, c := range s {
		if c == r {
			return true
		}
	}
	return false
}
