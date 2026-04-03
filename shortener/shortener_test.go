package shortener

import (
	"testing"
)

func TestGenerateLength(t *testing.T) {
	for range 100 {
		code := Generate()
		if len(code) != Length {
			t.Errorf("expected length %d, got %d for code %q", Length, len(code), code)
		}
	}
}

func TestGenerateCharset(t *testing.T) {
	for range 100 {
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

func TestGenerateUniqueness(t *testing.T) {
	const n = 10000
	seen := make(map[string]bool, n)
	for range n {
		code := Generate()
		if seen[code] {
			t.Errorf("duplicate code generated: %q", code)
		}
		seen[code] = true
	}
}

func TestGenerateCharDistribution(t *testing.T) {
	const n = 10000
	counts := make(map[rune]int, len(Charset))
	for range n {
		code := Generate()
		for _, c := range code {
			counts[c]++
		}
	}

	total := n * Length
	expected := total / len(Charset)
	threshold := expected / 3

	for _, c := range Charset {
		if counts[rune(c)] < threshold {
			t.Errorf("character %q under-represented: got %d, expected ~%d", c, counts[rune(c)], expected)
		}
	}
}
