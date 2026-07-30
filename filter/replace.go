// Package filter provides shared helpers used by the individual PII filters.
package filter

import (
	"strings"

	"github.com/openSUSE/piiplug/names"
)

const minNamelength = 8

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

	// Generate a new, unique pronounceable replacement.
	for {
		length := len(original)
		if length < minNamelength {
			length = minNamelength
		}
		rep, err := names.GeneratePronounceableName(names.WithLength(length))
		if err != nil {
			rep = "gen" + original
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
