package names

import (
	"regexp"
	"testing"
)

var shape = regexp.MustCompile(`^[a-z]+-[a-z]+(-\d+)?$`)

func TestGenerateShape(t *testing.T) {
	for range 200 {
		if n := Generate(nil); !shape.MatchString(n) {
			t.Fatalf("unexpected name %q", n)
		}
	}
}

func TestGenerateAvoidsTaken(t *testing.T) {
	seen := map[string]bool{}
	for range 500 {
		n := Generate(func(s string) bool { return seen[s] })
		if seen[n] {
			t.Fatalf("duplicate name %q", n)
		}
		seen[n] = true
	}
}

func TestGenerateFallsBackToSuffix(t *testing.T) {
	n := Generate(func(s string) bool { return !shape.MatchString(s) || len(s) < 100 && !containsDigit(s) })
	if !containsDigit(n) {
		t.Fatalf("expected numeric suffix, got %q", n)
	}
}

func containsDigit(s string) bool {
	for _, r := range s {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}
