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

func TestGenerateUniqueness(t *testing.T) {
	seen := make(map[string]bool)
	collisions := 0
	n := 1000
	for i := 0; i < n; i++ {
		code := Generate()
		if seen[code] {
			collisions++
		}
		seen[code] = true
	}
	// With 62^6 possibilities and 1000 samples, collisions should be extremely rare
	if collisions > 5 {
		t.Errorf("too many collisions: %d out of %d", collisions, n)
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
