package filter

import (
	"regexp"
	"sort"
	"strings"
)

// UniqueNamesFilter is the pure Go filter engine. It compiles a list of
// unique names into one case-insensitive regular expression and redacts and
// unredacts text against a shared replacement table. It has no dependency on
// the ADK; the plugin/callback layer lives in the piiplugin package.
type UniqueNamesFilter struct {
	Replacements *map[string]string
	Regex        *regexp.Regexp
}

// NewUniqueNamesFilter creates a UniqueNamesFilter over the given replacement
// table and name list. A nil table is replaced by a fresh one.
func NewUniqueNamesFilter(replacements *map[string]string, names []string) (*UniqueNamesFilter, error) {
	f := &UniqueNamesFilter{Replacements: replacements}
	if f.Replacements == nil {
		m := make(map[string]string)
		f.Replacements = &m
	}
	if err := f.InitRegex(names); err != nil {
		return nil, err
	}
	return f, nil
}

// InitRegex recompiles the filter's regex from the given name list. The names
// are sorted by length descending so that longer names match before their own
// prefixes.
func (f *UniqueNamesFilter) InitRegex(names []string) error {
	if len(names) == 0 {
		f.Regex = nil
		return nil
	}
	escaped := make([]string, len(names))
	for i, name := range names {
		escaped[i] = regexp.QuoteMeta(name)
	}
	sort.Slice(escaped, func(i, j int) bool {
		return len(escaped[i]) > len(escaped[j])
	})
	pattern := `\b(` + strings.Join(escaped, "|") + `)\b`
	var err error
	f.Regex, err = regexp.Compile("(?i)" + pattern)
	return err
}

// Redact replaces every matched name in text with a value from the shared
// replacement table. fullInput is the text the replacement is guaranteed not to
// collide with.
func (f *UniqueNamesFilter) Redact(text string, fullInput string) string {
	if f.Regex == nil {
		return text
	}
	return f.Regex.ReplaceAllStringFunc(text, func(match string) string {
		return GetReplacement(f.Replacements, match, fullInput)
	})
}

// Unredact reverses the redaction using the shared replacement table.
func (f *UniqueNamesFilter) Unredact(text string) string {
	return UnredactText(f.Replacements, text)
}
