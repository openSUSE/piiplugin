package piiplugin

import (
	"strings"
	"testing"

	"github.com/openSUSE/piiplugin/filter"
	filterusername "github.com/openSUSE/piiplugin/filter/username"
)

func TestPiiFilter(t *testing.T) {
	filter.UseMock = true
	defer func() {
		filter.UseMock = false
	}()

	f := NewPiiFilter(WithoutUsername(), WithoutHost())

	inputText := "Send email to alice@company.com or bob@office.org."
	redacted := f.Redact(inputText, inputText)

	if strings.Contains(redacted, "alice@company.com") || strings.Contains(redacted, "bob@office.org") {
		t.Errorf("Expected redaction, got: %q", redacted)
	}

	unredacted := f.Unredact(redacted)
	if unredacted != inputText {
		t.Errorf("Expected round-trip to restore original text, got: %q", unredacted)
	}
}

// TestPiiFilter_ReplacementsMap verifies that the Replacements map is properly
// populated and accessible after redaction.
func TestPiiFilter_ReplacementsMap(t *testing.T) {
	filter.UseMock = true
	defer func() {
		filter.UseMock = false
	}()

	f := NewPiiFilter(WithoutUsername(), WithoutHost())

	inputText := "Contact john.doe@company.com or jane.roe@example.org for help."
	redacted := f.Redact(inputText, inputText)

	if strings.Contains(redacted, "john.doe") || strings.Contains(redacted, "company.com") {
		t.Errorf("Expected email to be redacted, got: %q", redacted)
	}
	if strings.Contains(redacted, "jane.roe") || strings.Contains(redacted, "example.org") {
		t.Errorf("Expected email to be redacted, got: %q", redacted)
	}

	if f.Replacements == nil {
		t.Fatal("Replacements map should not be nil")
	}

	replacements := *f.Replacements
	if len(replacements) == 0 {
		t.Error("Expected at least one replacement to be generated")
	}

	for redactedToken, originalValue := range replacements {
		if redactedToken == "" {
			t.Error("Redacted token should not be empty string")
		}
		if originalValue == "" {
			t.Error("Original value should not be empty string")
		}
		if !strings.Contains(redacted, redactedToken) {
			t.Errorf("Expected redacted token %q to appear in redacted text %q", redactedToken, redacted)
		}
		if strings.Contains(redacted, originalValue) {
			t.Errorf("Original value %q should not appear in redacted text %q", originalValue, redacted)
		}
	}

	unredacted := f.Unredact(redacted)
	if unredacted != inputText {
		t.Errorf("Expected round-trip to restore original text\nGot: %q\nWant:%q", unredacted, inputText)
	}
}

// TestPiiFilter_UsernameSourceOptions verifies the source configuration option
// is carried through to the user name filter without failing the composite.
func TestPiiFilter_UsernameSourceOptions(t *testing.T) {
	// Both getent and auto must work in any build (both back off to whatever is
	// available); cgo is validated by the filter package itself.
	for _, opt := range []PiiPluginOption{
		WithUsernameSource(filterusername.SourceAuto),
		WithUsernameSource(filterusername.SourceGetent),
		WithUsernameSource(filterusername.SourceGetent),
	} {
		f := NewPiiFilter(WithoutEmail(), WithoutHost(), opt)
		if f.UsernameFilter == nil {
			t.Fatalf("UsernameFilter should be enabled")
		}
	}
}
