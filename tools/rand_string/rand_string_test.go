package rand_string

import (
	"strings"
	"testing"
)

func TestRandomLetter_LengthAndAlphabet(t *testing.T) {
	for _, n := range []int{1, 16, 64, 1024} {
		got := RandomLetter(n)
		if len(got) != n {
			t.Fatalf("RandomLetter(%d) returned len %d", n, len(got))
		}
		for _, r := range got {
			if !strings.ContainsRune(letterBytes, r) {
				t.Fatalf("char %q not in alphabet", r)
			}
		}
	}
}

func TestRandomInt_LengthAndAlphabet(t *testing.T) {
	got := RandomInt(32)
	if len(got) != 32 {
		t.Fatalf("len = %d", len(got))
	}
	for _, r := range got {
		if r < '0' || r > '9' {
			t.Fatalf("char %q not a digit", r)
		}
	}
}

func TestRandomBytes_CustomAlphabet(t *testing.T) {
	got := RandomBytes("ABC", 100)
	if len(got) != 100 {
		t.Fatalf("len = %d", len(got))
	}
	for _, r := range got {
		if r != 'A' && r != 'B' && r != 'C' {
			t.Fatalf("char %q outside alphabet", r)
		}
	}
}

func TestRandom_NonDeterministic(t *testing.T) {
	// Cryptographic randomness: two large samples MUST differ.
	a := RandomLetter(64)
	b := RandomLetter(64)
	if a == b {
		t.Fatalf("two 64-char samples collided: %q", a)
	}
}

func TestRandom_EdgeCases(t *testing.T) {
	if got := RandomLetter(0); got != "" {
		t.Fatalf("RandomLetter(0) = %q, want empty", got)
	}
	if got := RandomLetter(-1); got != "" {
		t.Fatalf("RandomLetter(-1) = %q, want empty", got)
	}
	if got := RandomBytes("", 10); got != "" {
		t.Fatalf("RandomBytes with empty alphabet = %q, want empty", got)
	}
}
