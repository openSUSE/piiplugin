package filter

import (
	"testing"

	"github.com/openSUSE/piiplug/names"
)

// TestGetReplacement_Basic verifies that an empty map generates a brand new replacement,
// saves it in the map, and returns it. It also checks length constraints.
func TestGetReplacement_Basic(t *testing.T) {
	replacements := make(map[string]string)
	original := "john"
	fullInput := "my name is john"

	rep := GetReplacement(&replacements, original, fullInput)

	if rep == "" {
		t.Fatal("Expected a non-empty generated replacement")
	}

	// Because len(original) is 4 which is less than minNamelength (8),
	// the returned replacement must be at least minNamelength (8) characters long.
	if len(rep) < minNamelength {
		t.Errorf("Expected replacement length to be at least %d, got %d (replacement: %q)", minNamelength, len(rep), rep)
	}

	// Verify that the mapping is saved correctly in the replacements map
	savedOriginal, exists := replacements[rep]
	if !exists {
		t.Errorf("Replacement %q was not saved in the replacements map", rep)
	}
	if savedOriginal != original {
		t.Errorf("Expected saved original for replacement %q to be %q, got %q", rep, original, savedOriginal)
	}
}

// TestGetReplacement_LengthConstraints verifies that for longer input strings,
// the replacement generated matches or exceeds the length of the original string.
func TestGetReplacement_LengthConstraints(t *testing.T) {
	replacements := make(map[string]string)
	original := "extremelylongname" // 17 chars long
	fullInput := "extremelylongname was here"

	rep := GetReplacement(&replacements, original, fullInput)

	if len(rep) < len(original) {
		t.Errorf("Expected replacement length to be at least %d (len of original), got %d (replacement: %q)", len(original), len(rep), rep)
	}
}

// TestGetReplacement_Consistency verifies that calling GetReplacement multiple times with the
// same original string returns the same replacement and does not add new mappings to the map.
func TestGetReplacement_Consistency(t *testing.T) {
	replacements := make(map[string]string)
	original := "alice"
	fullInput := "this is alice"

	rep1 := GetReplacement(&replacements, original, fullInput)
	rep2 := GetReplacement(&replacements, original, fullInput)

	if rep1 != rep2 {
		t.Errorf("Expected consistent replacements, but got %q and %q", rep1, rep2)
	}

	// Map should only contain exactly one entry
	if len(replacements) != 1 {
		t.Errorf("Expected map length to be 1, got %d", len(replacements))
	}
}

// TestGetReplacement_DotFiltering verifies that pre-existing replacement keys
// containing a dot "." (like domain name replacements or suffix mappings) are NOT
// returned as standard segment replacements.
func TestGetReplacement_DotFiltering(t *testing.T) {
	// Let's pre-populate the map with a key containing a "."
	replacements := map[string]string{
		"xyz.com": "gmail.com",
	}
	original := "gmail.com"
	fullInput := "contact gmail.com"

	rep := GetReplacement(&replacements, original, fullInput)

	// Since "xyz.com" contains a ".", it shouldn't be matched as a standard replacement for "gmail.com".
	// The function should generate a new replacement.
	if rep == "xyz.com" {
		t.Error("Expected GetReplacement to skip replacement keys containing '.' and generate a new one")
	}

	// The map should now contain two mappings (the original dot mapping and the new one)
	if len(replacements) != 2 {
		t.Errorf("Expected map to have 2 entries, got %d", len(replacements))
	}
}

// TestGetReplacement_PrefilledMap verifies that when a prefilled map is provided,
// special names configured within the map aren't replaced/redacted with newly generated ones,
// but instead preserve their pre-defined replacements.
func TestGetReplacement_PrefilledMap(t *testing.T) {
	// Define a prefilled map containing some custom rules:
	// 1. "SpecialAlice" has a prefilled custom placeholder "AlicePlaceholder".
	// 2. "SpecialBob" is whitelisted/mapped to itself ("SpecialBob"), so it remains unreplaced.
	replacements := map[string]string{
		"AlicePlaceholder": "SpecialAlice",
		"SpecialBob":       "SpecialBob",
	}

	fullInput := "Hello SpecialAlice and SpecialBob."

	// Case 1: Custom prefilled replacement for SpecialAlice
	repAlice := GetReplacement(&replacements, "SpecialAlice", fullInput)
	if repAlice != "AlicePlaceholder" {
		t.Errorf("Expected prefilled replacement 'AlicePlaceholder' for 'SpecialAlice', but got %q", repAlice)
	}

	// Case 2: Whitelisted SpecialBob mapped to itself so it is not replaced
	repBob := GetReplacement(&replacements, "SpecialBob", fullInput)
	if repBob != "SpecialBob" {
		t.Errorf("Expected prefilled replacement to keep 'SpecialBob' unchanged, but got %q", repBob)
	}

	// Verify that no extra replacements were added for these existing mappings
	if len(replacements) != 2 {
		t.Errorf("Expected map to still have exactly 2 entries, got %d", len(replacements))
	}
}

// TestGetReplacement_CollisionAvoidance verifies that the generator avoids collisions with
// both strings present in fullInput and existing keys in the replacements map.
func TestGetReplacement_CollisionAvoidance(t *testing.T) {
	replacements := make(map[string]string)
	original := "clash"

	// Let's mock a scenario with a simulated collision in the input.
	// Since GeneratePronounceableName is pseudo-random, let's generate a name first,
	// and then use it in fullInput to force a collision on a subsequent call.
	rep1 := GetReplacement(&replacements, original, "some input text")

	// Now try to get a replacement for a different original, but pass rep1 in fullInput.
	// The function must NOT return rep1 as a replacement for original2, because rep1 is in fullInput.
	original2 := "different"
	fullInputWithCollision := "this text contains " + rep1
	rep2 := GetReplacement(&replacements, original2, fullInputWithCollision)

	if rep1 == rep2 {
		t.Errorf("Collision avoidance failed: returned %q which is present in fullInput", rep2)
	}

	// Now verify that GetReplacement does not reuse any existing replacement keys in the map.
	// If a key is in replacements, even if not in fullInput, it shouldn't be chosen.
	// We'll generate a 3rd replacement and verify it is unique.
	original3 := "third"
	rep3 := GetReplacement(&replacements, original3, "some other text")

	if rep3 == rep1 || rep3 == rep2 {
		t.Errorf("Expected unique replacement, but got duplicate of an existing key: %q", rep3)
	}
}

// TestGetReplacement_Mocked verifies end-to-end integration with names.MockGenerator
// and the names.UseMock flag using the default PredictableMockGenerator behavior.
func TestGetReplacement_Mocked(t *testing.T) {
	names.UseMock = true
	defer func() {
		names.UseMock = false
	}()

	replacements := make(map[string]string)
	original := "john"
	fullInput := "my name is john"

	rep := GetReplacement(&replacements, original, fullInput)

	// Since "john" has length 4 which is < minNamelength (8),
	// we expect the mock to be called with length 8, returning "aaaaaaaa".
	expected := "aaaaaaaa"
	if rep != expected {
		t.Errorf("Expected mocked replacement %q, got %q", expected, rep)
	}
}
