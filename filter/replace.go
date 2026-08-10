// Package filter provides shared helpers used by the individual PII filters.
package filter

import (
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/openSUSE/piirplug/names"
)

const minNamelength = 8

// UseMock can be enabled during tests to return reversed original names
// instead of generating random pronounceable names.
var UseMock = false

// matchCasing maps the uppercase/lowercase pattern of the original string
// onto the replacement string.
func matchCasing(original, replacement string) string {
	origRunes := []rune(original)
	repRunes := []rune(replacement)
	result := make([]rune, len(repRunes))

	// Check if original is all uppercase (and has at least one letter)
	allUpper := true
	hasLetter := false
	for _, r := range origRunes {
		if unicode.IsLetter(r) {
			hasLetter = true
			if !unicode.IsUpper(r) {
				allUpper = false
			}
		}
	}
	if hasLetter && allUpper {
		for i, r := range repRunes {
			result[i] = unicode.ToUpper(r)
		}
		return string(result)
	}

	// Check if original is all lowercase
	allLower := true
	hasLetter = false
	for _, r := range origRunes {
		if unicode.IsLetter(r) {
			hasLetter = true
			if !unicode.IsLower(r) {
				allLower = false
			}
		}
	}
	if hasLetter && allLower {
		for i, r := range repRunes {
			result[i] = unicode.ToLower(r)
		}
		return string(result)
	}

	// Otherwise apply letter-by-letter casing
	for i, r := range repRunes {
		if i < len(origRunes) {
			origRune := origRunes[i]
			if unicode.IsUpper(origRune) {
				result[i] = unicode.ToUpper(r)
			} else if unicode.IsLower(origRune) {
				result[i] = unicode.ToLower(r)
			} else {
				result[i] = r
			}
		} else {
			result[i] = r
		}
	}
	return string(result)
}

// GetReplacement retrieves an existing replacement for original from the shared
// replacement table, or generates a new replacement word.
//
// The table is a *map[string]string whose key is the generated replacement and
// whose value is the original string. Using a pointer to a single map allows the
// table to be shared across multiple filters so that redaction and the reverse
// unredaction stay consistent between them.
//
// fullInput is the complete input text; generated replacements are guaranteed
// not to appear in it, preventing accidental collisions with real input.
func GetReplacement(replacements *map[string]string, original string, fullInput string) string {
	m := *replacements

	// Return an existing replacement if we already mapped this original string exactly.
	// Keys containing "." are combined segment+suffix mappings and must not be
	// returned as a pure segment replacement.
	for rep, orig := range m {
		if orig == original && !strings.Contains(rep, ".") {
			return rep
		}
	}

	// If not matched exactly, try case-insensitive lookup to preserve casing consistent with the original mapping.
	for rep, orig := range m {
		if strings.ToLower(orig) == strings.ToLower(original) && !strings.Contains(rep, ".") {
			matchedRep := matchCasing(original, rep)
			m[matchedRep] = original
			return matchedRep
		}
	}

	if UseMock {
		runes := []rune(original)
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		rep := string(runes)
		m[rep] = original
		return rep
	}

	// Generate a new, unique pronounceable replacement.
	for {
		length := max(len(original), minNamelength)
		length = min(length, 255)
		rep, err := names.GeneratePronounceableName(names.WithLength(length))
		if err != nil {
			// ignore error as for this function it can opnly be maximal length
			rep, _ = names.GenerateRandomString(names.WithLength(length))
		}

		matchedRep := matchCasing(original, rep)

		// Make sure the generated word isn't part of the input and isn't
		// already used as a replacement in the shared table.
		if strings.Contains(fullInput, matchedRep) {
			continue
		}
		if _, exists := m[matchedRep]; exists {
			continue
		}
		m[matchedRep] = original
		return matchedRep
	}
}

// UnredactText replaces all occurrences of replacement keys in text with their original values in a single pass.
func UnredactText(replacements *map[string]string, text string) string {
	if replacements == nil || len(*replacements) == 0 {
		return text
	}
	m := *replacements

	// Extract and sort keys by length descending to ensure longer patterns match first
	keys := make([]string, 0, len(m))
	for k := range m {
		if k != "" {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return text
	}
	sort.Slice(keys, func(i, j int) bool {
		return len(keys[i]) > len(keys[j])
	})

	// Build a regex matching any of the keys.
	// Since keys might contain regex special characters, we must use QuoteMeta.
	escapedKeys := make([]string, len(keys))
	for i, k := range keys {
		escapedKeys[i] = regexp.QuoteMeta(k)
	}
	pattern := strings.Join(escapedKeys, "|")
	re, err := regexp.Compile(pattern)
	if err != nil {
		// Fallback to serial replacement if regex compilation fails (should never happen for valid QuoteMeta)
		for _, k := range keys {
			text = strings.ReplaceAll(text, k, m[k])
		}
		return text
	}

	return re.ReplaceAllStringFunc(text, func(match string) string {
		if orig, ok := m[match]; ok {
			return orig
		}
		return match
	})
}
