package flag

import (
	"testing"
)

func TestSeedDeterministic(t *testing.T) {
	s1 := Seed(42, "web-xss", nil)
	s2 := Seed(42, "web-xss", nil)
	if string(s1) != string(s2) {
		t.Fatal("seed must be deterministic")
	}
}

func TestSeedDifferent(t *testing.T) {
	s1 := Seed(42, "web-xss", nil)
	s2 := Seed(43, "web-xss", nil)
	if string(s1) == string(s2) {
		t.Fatal("different users must produce different seeds")
	}
}

func TestGeneratePreservesNonLetters(t *testing.T) {
	base := "X{X_123!}"
	seed := Seed(1, "test", nil)
	result := Generate(base, seed, nil)
	if result[1:2] != "{" {
		t.Fatalf("expected { preserved, got %s", result[1:2])
	}
	if result[len(result)-1:] != "}" {
		t.Fatalf("expected }} preserved, got %s", result[len(result)-1:])
	}
	if result[3:7] != "_123" {
		t.Fatalf("expected _123 preserved, got %s", result[3:7])
	}
	if result[7:8] != "!" {
		t.Fatalf("expected ! preserved, got %s", result[7:8])
	}
}

func TestGenerateChangesLetters(t *testing.T) {
	base := "CTF{aa}"
	seed := make([]byte, 32)
	for i := range seed {
		seed[i] = 1
	}
	result := Generate(base, seed, nil)
	if result == base {
		t.Fatal("expected transformation with seed=1, got same flag")
	}
}

func TestGenerateWithOverrides(t *testing.T) {
	base := "X{X{a}"
	seed := make([]byte, 32)
	for i := range seed {
		seed[i] = 1
	}
	overrides := map[byte][]string{'a': {"9"}}
	result := Generate(base, seed, overrides)
	if result != "x{x{9}" {
		t.Fatalf("expected x{{x{{9}} with override, got %s", result)
	}
}

func TestGenerateStaticWithZeroSeed(t *testing.T) {
	base := "CTF{abcdefghijklmnopqrstuvwxyz}"
	seed := make([]byte, 32)
	result := Generate(base, seed, nil)
	if result != base {
		t.Fatalf("seed=0 should keep all letters unchanged, got %s", result)
	}
}

func TestGeneratePreservesNumbers(t *testing.T) {
	base := "XXX{1234567890}"
	seed := make([]byte, 32)
	for i := range seed {
		seed[i] = 1
	}
	result := Generate(base, seed, nil)
	expected := "xxx{1234567890}"
	if result != expected {
		t.Fatalf("numbers must be preserved, got %s", result)
	}
}
