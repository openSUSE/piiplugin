package names

import (
	"math/rand"
	"strings"
	"testing"
)

func TestGeneratePronounceableName(t *testing.T) {
	// Test basic length constraints
	lengths := []int{5, 8, 12, 16, 24, 32}
	for _, l := range lengths {
		name, err := GeneratePronounceableName(WithLength(l))
		if err != nil {
			t.Fatalf("Failed to generate name of length %d: %v", l, err)
		}
		if len(name) != l {
			t.Errorf("Expected name of length %d, got %d (name: %q)", l, len(name), name)
		}
		// Confirm the name only contains valid characters
		for _, c := range name {
			if !strings.ContainsRune("abcdefghijklmnopqrstuvwxyz", c) {
				t.Errorf("Name %q contains unexpected character %q", name, c)
			}
		}
	}

	// Test invalid lengths
	invalidLengths := []int{-1, 0, 256}
	for _, l := range invalidLengths {
		name, err := GeneratePronounceableName(WithLength(l))
		if err == nil {
			t.Errorf("Expected error for invalid length %d, but got none (name: %q)", l, name)
		}
	}
}

func TestGeneratePronounceableName_Stress(t *testing.T) {
	// Generate 1000 names of random lengths between 1 and 50
	for i := 0; i < 1000; i++ {
		length := (i % 50) + 1
		name, err := GeneratePronounceableName(WithLength(length))
		if err != nil {
			t.Fatalf("Failed to generate name of length %d at iteration %d: %v", length, i, err)
		}
		if len(name) != length {
			t.Errorf("Expected name of length %d, got %d (name: %q)", length, len(name), name)
		}
	}
}

func TestGeneratePronounceableName_Options(t *testing.T) {
	// Test default length (should be 10)
	name, err := GeneratePronounceableName()
	if err != nil {
		t.Fatalf("Failed to generate name with default options: %v", err)
	}
	if len(name) != 10 {
		t.Errorf("Expected default name length of 10, got %d", len(name))
	}

	// Test WithSeed (determinism)
	seed := int64(12345)
	name1, err := GeneratePronounceableName(WithLength(12), WithSeed(seed))
	if err != nil {
		t.Fatalf("Failed to generate with seed: %v", err)
	}
	name2, err := GeneratePronounceableName(WithLength(12), WithSeed(seed))
	if err != nil {
		t.Fatalf("Failed to generate with seed: %v", err)
	}
	if name1 != name2 {
		t.Errorf("Expected deterministic names for same seed, got %q and %q", name1, name2)
	}

	// Test WithRand (custom *rand.Rand generator)
	src := rand.NewSource(999)
	rnd := rand.New(src)
	name3, err := GeneratePronounceableName(WithLength(8), WithRand(rnd))
	if err != nil {
		t.Fatalf("Failed to generate with custom rand: %v", err)
	}
	if len(name3) != 8 {
		t.Errorf("Expected name of length 8 with custom rand, got %d", len(name3))
	}
}

type lcgSource struct {
	state uint32
}

func (s *lcgSource) Int63() int64 {
	s.state = s.state*1103515245 + 12345
	return int64(s.state & 0x7fffffff)
}

func (s *lcgSource) Seed(seed int64) {}

func TestGenerateRandomString(t *testing.T) {
	// Test basic length constraints
	lengths := []int{5, 8, 12, 16, 24, 32}
	for _, l := range lengths {
		str, err := GenerateRandomString(WithLength(l))
		if err != nil {
			t.Fatalf("Failed to generate random string of length %d: %v", l, err)
		}
		if len(str) != l {
			t.Errorf("Expected random string of length %d, got %d (string: %q)", l, len(str), str)
		}
		// Confirm the string only contains valid characters
		for _, c := range str {
			if !strings.ContainsRune("abcdefghijklmnopqrstuvwxyz", c) {
				t.Errorf("String %q contains unexpected character %q", str, c)
			}
		}
	}

	// Test invalid lengths
	invalidLengths := []int{-1, 0, 256}
	for _, l := range invalidLengths {
		str, err := GenerateRandomString(WithLength(l))
		if err == nil {
			t.Errorf("Expected error for invalid length %d, but got none (string: %q)", l, str)
		}
	}

	// Test default length (should be 10)
	str, err := GenerateRandomString()
	if err != nil {
		t.Fatalf("Failed to generate random string with default options: %v", err)
	}
	if len(str) != 10 {
		t.Errorf("Expected default string length of 10, got %d", len(str))
	}

	// Test WithSeed (determinism)
	seed := int64(12345)
	str1, err := GenerateRandomString(WithLength(12), WithSeed(seed))
	if err != nil {
		t.Fatalf("Failed to generate random string with seed: %v", err)
	}
	str2, err := GenerateRandomString(WithLength(12), WithSeed(seed))
	if err != nil {
		t.Fatalf("Failed to generate random string with same seed: %v", err)
	}
	if str1 != str2 {
		t.Errorf("Expected deterministic random strings for same seed, got %q and %q", str1, str2)
	}

	// Test WithRand (custom *rand.Rand generator)
	src := rand.NewSource(999)
	rnd := rand.New(src)
	str3, err := GenerateRandomString(WithLength(8), WithRand(rnd))
	if err != nil {
		t.Fatalf("Failed to generate random string with custom rand: %v", err)
	}
	if len(str3) != 8 {
		t.Errorf("Expected random string of length 8 with custom rand, got %d", len(str3))
	}
}
