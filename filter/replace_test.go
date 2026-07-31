package filter

import (
	"testing"
	"unicode"
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

// TestGetReplacement_Mocked verifies end-to-end mock behavior where names are reversed
// when UseMock is enabled.
func TestGetReplacement_Mocked(t *testing.T) {
	UseMock = true
	defer func() {
		UseMock = false
	}()

	replacements := make(map[string]string)

	// Test 1: "foo" -> "oof"
	repFoo := GetReplacement(&replacements, "foo", "hello foo")
	if repFoo != "oof" {
		t.Errorf("Expected 'oof', got %q", repFoo)
	}

	// Test 2: "alice" -> "ecila"
	repAlice := GetReplacement(&replacements, "alice", "hello alice")
	if repAlice != "ecila" {
		t.Errorf("Expected 'ecila', got %q", repAlice)
	}
}

// TestGetReplacement_CaseMatching verifies that the replacements preserve and match
// the casing of the original input string.
func TestGetReplacement_CaseMatching(t *testing.T) {
	replacements := make(map[string]string)

	// Pre-fill the map: "teken" -> "mechen", "baar" -> "foo"
	replacements["teken"] = "mechen"
	replacements["baar"] = "foo"

	// 1. "Mechen" should be replaced by "Teken" (Capitalized first letter)
	rep1 := GetReplacement(&replacements, "Mechen", "Hello Mechen")
	if rep1 != "Teken" {
		t.Errorf("Expected 'Teken' for 'Mechen' with pre-filled 'teken' -> 'mechen', got %q", rep1)
	}

	// 2. "FoO" should be replaced by "BaAr" (Alternate uppercase/lowercase)
	rep2 := GetReplacement(&replacements, "FoO", "Hello FoO")
	if rep2 != "BaAr" {
		t.Errorf("Expected 'BaAr' for 'FoO' with pre-filled 'baar' -> 'foo', got %q", rep2)
	}

	// 3. "foo" should be replaced by "baar" (exact match/all lowercase)
	rep3 := GetReplacement(&replacements, "foo", "Hello foo")
	if rep3 != "baar" {
		t.Errorf("Expected 'baar' for 'foo', got %q", rep3)
	}

	// 4. "FOO" should be replaced by "BAAR" (all uppercase)
	rep4 := GetReplacement(&replacements, "FOO", "Hello FOO")
	if rep4 != "BAAR" {
		t.Errorf("Expected 'BAAR' for 'FOO', got %q", rep4)
	}

	// 5. Test generating a brand new word with specific casing: "Alice"
	// Because len("Alice") is 5 (less than minNameLength 8), the generated name
	// will be longer, but we should verify that it matches the casing structure correctly.
	repAlice := GetReplacement(&replacements, "Alice", "Hello Alice")
	if len(repAlice) == 0 {
		t.Fatal("Expected non-empty replacement for 'Alice'")
	}
	runes := []rune(repAlice)
	if !unicode.IsUpper(runes[0]) {
		t.Errorf("Expected replacement for 'Alice' to start with an uppercase letter, got %q", repAlice)
	}
	for i := 1; i < len(runes); i++ {
		// Since "Alice" has length 5: A (upper), l, i, c, e (lower).
		// Any index >= 5 keeps the replacement's original casing.
		// For index 1 to 4, they must be lowercase since 'l', 'i', 'c', 'e' are lowercase.
		if i < 5 && !unicode.IsLower(runes[i]) {
			t.Errorf("Expected replacement rune at index %d to be lowercase, got %c in %q", i, runes[i], repAlice)
		}
	}
}
