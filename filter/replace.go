// Package filter provides shared helpers used by the individual PII filters.
package filter

import (
	"strings"

	"github.com/openSUSE/piiplug/names"
)

const minNamelength = 8

// UseMock can be enabled during tests to return reversed original names
// instead of generating random pronounceable names.
var UseMock = false

// GetReplacement retrieves an existing replacement for original from the shared
// replacement table, or generates a new, unique pronounceable replacement word.
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

	// Return an existing replacement if we already mapped this original string.
	// Keys containing "." are combined segment+suffix mappings and must not be
	// returned as a pure segment replacement.
	for rep, orig := range m {
		if orig == original && !strings.Contains(rep, ".") {
			return rep
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

		// Make sure the generated word isn't part of the input and isn't
		// already used as a replacement in the shared table.
		if strings.Contains(fullInput, rep) {
			continue
		}
		if _, exists := m[rep]; exists {
			continue
		}
		m[rep] = original
		return rep
	}
}
