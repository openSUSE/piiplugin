package filteremail

import (
	"strings"
	"testing"
)

// newReplMap returns a pointer to an empty shared replacement map for tests.
func newReplMap() *map[string]string {
	m := make(map[string]string)
	return &m
}

func newTestFilter() *EmailFilter {
	return NewEmailFilter(WithTLDSuffix(map[string]string{"*": "tld"}))
}

func TestNewEmailFilter_Defaults(t *testing.T) {
	f := NewEmailFilter()
	if f == nil {
		t.Fatal("Filter should not be nil")
	}
}

func TestEmailFilter_RedactAndUnredact_Default(t *testing.T) {
	f := newTestFilter()

	inputText := "Contact us at john.doe@gmail.com or support@company.org."
	redacted := f.Redact(inputText, inputText)

	if strings.Contains(redacted, "john.doe") || strings.Contains(redacted, "gmail.com") {
		t.Errorf("Redacted text still contains original email details: %q", redacted)
	}
	if !strings.Contains(redacted, "@") {
		t.Errorf("Redacted text should still contain '@': %q", redacted)
	}
	if !strings.Contains(redacted, ".tld") {
		t.Errorf("Expected TLD to be replaced with '.tld': %q", redacted)
	}

	unredacted := f.Unredact(redacted)
	if unredacted != inputText {
		t.Errorf("Unredacting failed.\nExpected: %q\nGot:      %q", inputText, unredacted)
	}
}

func TestEmailFilter_SpecificTLDSuffixTable(t *testing.T) {
	f := NewEmailFilter(WithTLDSuffix(map[string]string{"org": "orgtld"}))

	inputText := "Emails: test.user@company.org and admin@gmail.com"
	redacted := f.Redact(inputText, inputText)

	if strings.Contains(redacted, "company.org") {
		t.Errorf("company.org should have been redacted: %q", redacted)
	}
	if !strings.Contains(redacted, ".orgtld") {
		t.Errorf("org TLD should have been replaced with .orgtld: %q", redacted)
	}
	if !strings.Contains(redacted, "com") {
		t.Errorf("com TLD should have been kept: %q", redacted)
	}
	if strings.Contains(redacted, "gmail.com") {
		t.Errorf("gmail name should have been redacted: %q", redacted)
	}

	unredacted := f.Unredact(redacted)
	if unredacted != inputText {
		t.Errorf("Unredacting with specific TLD table failed.\nExpected: %q\nGot:      %q", inputText, unredacted)
	}
}

func TestEmailFilter_InputCollisionAvoidance(t *testing.T) {
	f := newTestFilter()

	inputText := "Our email is test@company.com. Do not generate 'test' or 'company' or any word in this sentence."
	redacted := f.Redact(inputText, inputText)

	if strings.Contains(redacted, "test@") {
		t.Errorf("Expected redacted email, but got: %q", redacted)
	}

	for rep := range *f.replacements {
		cleanRep := rep
		if idx := strings.Index(rep, "."); idx != -1 {
			cleanRep = rep[:idx]
		}
		if strings.Contains(inputText, cleanRep) {
			t.Errorf("Generated replacement %q was found in the input text!", cleanRep)
		}
	}
}

func TestEmailFilter_MultipleSubdomains(t *testing.T) {
	f := newTestFilter()

	inputText := "Send details to support@sub.domain.co.uk or admin@internal.system.com"
	redacted := f.Redact(inputText, inputText)

	unredacted := f.Unredact(redacted)
	if unredacted != inputText {
		t.Errorf("Unredacting multiple subdomains failed.\nExpected: %q\nGot:      %q", inputText, unredacted)
	}
}

func TestEmailFilter_NoTLD(t *testing.T) {
	f := NewEmailFilter(WithTLDSuffix(map[string]string{"*": "tld"}))

	inputText := "cron[123]: mail from goo@baar delivered to root@localhost"
	redacted := f.Redact(inputText, inputText)

	for _, orig := range []string{"goo@baar", "root@localhost"} {
		if strings.Contains(redacted, orig) {
			t.Errorf("Address %q was not redacted: %q", orig, redacted)
		}
	}
	if strings.Contains(redacted, ".tld") {
		t.Errorf("TLD-less address must not get a TLD suffix appended: %q", redacted)
	}

	unredacted := f.Unredact(redacted)
	if unredacted != inputText {
		t.Errorf("Unredacting TLD-less addresses failed.\nExpected: %q\nGot:      %q", inputText, unredacted)
	}
}

func TestEmailFilter_MixedTLDAndNoTLD(t *testing.T) {
	f := newTestFilter()

	inputText := "Forwarded from admin@mailhost to jane.doe@example.com successfully."
	redacted := f.Redact(inputText, inputText)

	if strings.Contains(redacted, "admin@mailhost") || strings.Contains(redacted, "jane.doe@example.com") {
		t.Errorf("Text was not fully redacted: %q", redacted)
	}
	if !strings.Contains(redacted, ".tld") {
		t.Errorf("Expected the address with a TLD to be suffixed with '.tld': %q", redacted)
	}

	unredacted := f.Unredact(redacted)
	if unredacted != inputText {
		t.Errorf("Unredacting mixed addresses failed.\nExpected: %q\nGot:      %q", inputText, unredacted)
	}
}

func TestEmailFilter_NoTLDSubdomains(t *testing.T) {
	f := newTestFilter()

	inputText := "login for svc.acct@build.node.local1 and ops@10.0.0.5 recorded"
	redacted := f.Redact(inputText, inputText)

	for _, orig := range []string{"svc.acct@build.node.local1", "ops@10.0.0.5"} {
		if strings.Contains(redacted, orig) {
			t.Errorf("Address %q was not redacted: %q", orig, redacted)
		}
	}

	unredacted := f.Unredact(redacted)
	if unredacted != inputText {
		t.Errorf("Unredacting non-TLD hosts failed.\nExpected: %q\nGot:      %q", inputText, unredacted)
	}
}

func TestSplitDomain(t *testing.T) {
	tests := []struct {
		domain     string
		wantLabels []string
		wantTLD    string
	}{
		{"example.com", []string{"example"}, "com"},
		{"sub.domain.co.uk", []string{"sub", "domain", "co"}, "uk"},
		{"baar", []string{"baar"}, ""},
		{"localhost", []string{"localhost"}, ""},
		{"build.node.local1", []string{"build", "node", "local1"}, ""},
		{"10.0.0.5", []string{"10", "0", "0", "5"}, ""},
	}
	for _, tc := range tests {
		labels, tld := splitDomain(tc.domain)
		if tld != tc.wantTLD {
			t.Errorf("splitDomain(%q) tld = %q, want %q", tc.domain, tld, tc.wantTLD)
		}
		if strings.Join(labels, ".") != strings.Join(tc.wantLabels, ".") {
			t.Errorf("splitDomain(%q) labels = %v, want %v", tc.domain, labels, tc.wantLabels)
		}
	}
}

func TestEmailFilter_FunctionalOptions(t *testing.T) {
	if f := NewEmailFilter(WithTLDSuffix(map[string]string{"*": ""})); f == nil {
		t.Fatal("Filter should not be nil")
	}
	if f := NewEmailFilter(WithTLDSuffix(map[string]string{"io": "iotld", "org": "orgtld"})); f == nil {
		t.Fatal("Filter should not be nil")
	}
}

func TestEmailFilter_WithReplacement_SharedMap(t *testing.T) {
	shared := map[string]string{}
	f := NewEmailFilter(WithTLDSuffix(map[string]string{"*": "tld"}), WithReplacement(&shared))

	inputText := "Reach me at jane.roe@example.org"
	redacted := f.Redact(inputText, inputText)

	if len(shared) == 0 {
		t.Fatal("Expected shared replacement map to be populated by the filter")
	}
	if strings.Contains(redacted, "jane.roe") || strings.Contains(redacted, "example.org") {
		t.Errorf("Redacted text still contains original details: %q", redacted)
	}

	other := NewEmailFilter(WithReplacement(&shared))
	if got := other.Unredact(redacted); got != inputText {
		t.Errorf("Shared map did not round-trip.\nExpected: %q\nGot:      %q", inputText, got)
	}
}

func TestEmailFilter_WithReplacementOption(t *testing.T) {
	shared := map[string]string{}
	f := NewEmailFilter(WithReplacement(&shared))

	redacted := f.Redact("Mail: sam.smith@corp.io", "Mail: sam.smith@corp.io")
	if len(shared) == 0 {
		t.Fatal("Expected the passed-in shared map to be populated via WithReplacement")
	}
	if strings.Contains(redacted, "sam.smith") || strings.Contains(redacted, "corp.io") {
		t.Errorf("Redacted text still contains original details: %q", redacted)
	}
}
