package filteremail

import (
	"strings"
	"testing"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"
)

// mockContext implements agent.Context for testing callbacks
type mockContext struct {
	agent.Context
}

// newReplMap returns a pointer to an empty shared replacement map for tests.
func newReplMap() *map[string]string {
	m := make(map[string]string)
	return &m
}

func TestNewEmailPlugin_Defaults(t *testing.T) {
	plugin, err := NewEmailPlugin()
	if err != nil {
		t.Fatalf("Failed to create EmailPlugin: %v", err)
	}
	if plugin == nil {
		t.Fatal("Plugin should not be nil")
	}
}

func TestEmailPlugin_RedactAndUnredact_Default(t *testing.T) {
	// With default settings, tldSuffix should be map[string]string{"*": "tld"}
	p := &EmailPlugin{
		replacements: newReplMap(),
		tldSuffix:    map[string]string{"*": "tld"},
	}

	inputText := "Contact us at john.doe@gmail.com or support@company.org."
	redacted := p.redactEmails(inputText, inputText)

	if strings.Contains(redacted, "john.doe") || strings.Contains(redacted, "gmail.com") {
		t.Errorf("Redacted text still contains original email details: %q", redacted)
	}
	if !strings.Contains(redacted, "@") {
		t.Errorf("Redacted text should still contain '@': %q", redacted)
	}
	if !strings.Contains(redacted, ".tld") {
		t.Errorf("Expected TLD to be replaced with '.tld': %q", redacted)
	}

	unredacted := p.unredactText(redacted)
	if unredacted != inputText {
		t.Errorf("Unredacting failed.\nExpected: %q\nGot:      %q", inputText, unredacted)
	}
}

func TestEmailPlugin_SpecificTLDSuffixTable(t *testing.T) {
	// Configured with a replacement table:
	// org TLD is replaced as "orgtld", and other TLDs fall back to keeping original
	p := &EmailPlugin{
		replacements: newReplMap(),
		tldSuffix:    map[string]string{"org": "orgtld"},
	}

	inputText := "Emails: test.user@company.org and admin@gmail.com"
	redacted := p.redactEmails(inputText, inputText)

	// test.user@company.org should have its TLD replaced with "orgtld"
	// admin@gmail.com should keep its "com" TLD
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

	unredacted := p.unredactText(redacted)
	if unredacted != inputText {
		t.Errorf("Unredacting with specific TLD table failed.\nExpected: %q\nGot:      %q", inputText, unredacted)
	}
}

func TestEmailPlugin_InputCollisionAvoidance(t *testing.T) {
	// If the name generator wants to generate a word that is already part of the input,
	// it should keep generating until it finds a unique one.
	p := &EmailPlugin{
		replacements: newReplMap(),
		tldSuffix:    map[string]string{"*": "tld"},
	}

	inputText := "Our email is test@company.com. Do not generate 'test' or 'company' or any word in this sentence."
	redacted := p.redactEmails(inputText, inputText)

	if strings.Contains(redacted, "test@") {
		t.Errorf("Expected redacted email, but got: %q", redacted)
	}

	for rep := range *p.replacements {
		// Strip .tld or other dot suffixes from combined domain parts if mapped with it
		cleanRep := rep
		if idx := strings.Index(rep, "."); idx != -1 {
			cleanRep = rep[:idx]
		}
		if strings.Contains(inputText, cleanRep) {
			t.Errorf("Generated replacement %q was found in the input text!", cleanRep)
		}
	}
}

func TestEmailPlugin_MultipleSubdomains(t *testing.T) {
	p := &EmailPlugin{
		replacements: newReplMap(),
		tldSuffix:    map[string]string{"*": "tld"},
	}

	inputText := "Send details to support@sub.domain.co.uk or admin@internal.system.com"
	redacted := p.redactEmails(inputText, inputText)

	unredacted := p.unredactText(redacted)
	if unredacted != inputText {
		t.Errorf("Unredacting multiple subdomains failed.\nExpected: %q\nGot:      %q", inputText, unredacted)
	}
}

func TestEmailPlugin_NoTLD(t *testing.T) {
	// Addresses without a TLD, as they occur in local logs, must be redacted too.
	p := &EmailPlugin{
		replacements: newReplMap(),
		tldSuffix:    map[string]string{"*": "tld"},
	}

	inputText := "cron[123]: mail from goo@baar delivered to root@localhost"
	redacted := p.redactEmails(inputText, inputText)

	for _, orig := range []string{"goo@baar", "root@localhost"} {
		if strings.Contains(redacted, orig) {
			t.Errorf("Address %q was not redacted: %q", orig, redacted)
		}
	}
	// The shape must be preserved: no ".tld" may be appended to a TLD-less address.
	if strings.Contains(redacted, ".tld") {
		t.Errorf("TLD-less address must not get a TLD suffix appended: %q", redacted)
	}

	unredacted := p.unredactText(redacted)
	if unredacted != inputText {
		t.Errorf("Unredacting TLD-less addresses failed.\nExpected: %q\nGot:      %q", inputText, unredacted)
	}
}

func TestEmailPlugin_MixedTLDAndNoTLD(t *testing.T) {
	// A TLD-less and a regular address in the same text must both round-trip.
	p := &EmailPlugin{
		replacements: newReplMap(),
		tldSuffix:    map[string]string{"*": "tld"},
	}

	inputText := "Forwarded from admin@mailhost to jane.doe@example.com successfully."
	redacted := p.redactEmails(inputText, inputText)

	if strings.Contains(redacted, "admin@mailhost") || strings.Contains(redacted, "jane.doe@example.com") {
		t.Errorf("Text was not fully redacted: %q", redacted)
	}
	if !strings.Contains(redacted, ".tld") {
		t.Errorf("Expected the address with a TLD to be suffixed with '.tld': %q", redacted)
	}

	unredacted := p.unredactText(redacted)
	if unredacted != inputText {
		t.Errorf("Unredacting mixed addresses failed.\nExpected: %q\nGot:      %q", inputText, unredacted)
	}
}

func TestEmailPlugin_NoTLDSubdomains(t *testing.T) {
	// A dotted host whose last label is not a TLD must have every label replaced.
	p := &EmailPlugin{
		replacements: newReplMap(),
		tldSuffix:    map[string]string{"*": "tld"},
	}

	inputText := "login for svc.acct@build.node.local1 and ops@10.0.0.5 recorded"
	redacted := p.redactEmails(inputText, inputText)

	for _, orig := range []string{"svc.acct@build.node.local1", "ops@10.0.0.5"} {
		if strings.Contains(redacted, orig) {
			t.Errorf("Address %q was not redacted: %q", orig, redacted)
		}
	}

	unredacted := p.unredactText(redacted)
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

func TestEmailPlugin_FunctionalOptions(t *testing.T) {
	// Test setting empty option which should fall back to "*"
	plugin, err := NewEmailPlugin(WithTLDSuffix(map[string]string{"*": ""}))
	if err != nil {
		t.Fatalf("Failed to create with empty suffix: %v", err)
	}
	if plugin == nil {
		t.Fatal("Plugin should not be nil")
	}

	// Test specific suffix table configuration
	plugin2, err := NewEmailPlugin(WithTLDSuffix(map[string]string{"io": "iotld", "org": "orgtld"}))
	if err != nil {
		t.Fatalf("Failed to create with specific suffix table: %v", err)
	}
	if plugin2 == nil {
		t.Fatal("Plugin2 should not be nil")
	}
}

func TestEmailPlugin_WithReplacement_SharedMap(t *testing.T) {
	// A prefilled, shared replacement map should be reused and extended in place,
	// so callers (and other filters) observe the mappings the filter creates.
	shared := map[string]string{}
	p := &EmailPlugin{
		replacements: &shared,
		tldSuffix:    map[string]string{"*": "tld"},
	}

	inputText := "Reach me at jane.roe@example.org"
	redacted := p.redactEmails(inputText, inputText)

	if len(shared) == 0 {
		t.Fatal("Expected shared replacement map to be populated by the filter")
	}
	if strings.Contains(redacted, "jane.roe") || strings.Contains(redacted, "example.org") {
		t.Errorf("Redacted text still contains original details: %q", redacted)
	}

	// A second plugin sharing the same map must be able to reverse the redaction.
	other := &EmailPlugin{
		replacements: &shared,
		tldSuffix:    map[string]string{"*": "tld"},
	}
	if got := other.unredactText(redacted); got != inputText {
		t.Errorf("Shared map did not round-trip.\nExpected: %q\nGot:      %q", inputText, got)
	}
}

func TestEmailPlugin_WithReplacementOption(t *testing.T) {
	shared := map[string]string{}
	plugin, err := NewEmailPlugin(WithReplacement(&shared))
	if err != nil {
		t.Fatalf("Failed to create EmailPlugin with WithReplacement: %v", err)
	}
	if plugin == nil {
		t.Fatal("Plugin should not be nil")
	}

	req := &model.LLMRequest{
		Contents: []*genai.Content{
			{Parts: []*genai.Part{{Text: "Mail: sam.smith@corp.io"}}},
		},
	}
	if _, err = plugin.BeforeModelCallback()(&mockContext{}, req); err != nil {
		t.Fatalf("BeforeModelCallback failed: %v", err)
	}
	if len(shared) == 0 {
		t.Fatal("Expected the passed-in shared map to be populated via WithReplacement")
	}
}

func TestEmailPlugin_Callbacks(t *testing.T) {
	// Test the plugin lifecycle callbacks with LLMRequest and LLMResponse.
	p, err := NewEmailPlugin(WithTLDSuffix(map[string]string{"*": "com"}))
	if err != nil {
		t.Fatalf("Failed to create EmailPlugin: %v", err)
	}

	req := &model.LLMRequest{
		Contents: []*genai.Content{
			{
				Parts: []*genai.Part{
					{
						Text: "Help desk email is help.desk@support.com. Please write to them.",
					},
				},
			},
		},
	}

	ctx := &mockContext{}

	// Run BeforeModelCallback to redact
	_, err = p.BeforeModelCallback()(ctx, req)
	if err != nil {
		t.Fatalf("BeforeModelCallback failed: %v", err)
	}

	redactedText := req.Contents[0].Parts[0].Text
	if strings.Contains(redactedText, "help.desk") || strings.Contains(redactedText, "support.com") {
		t.Errorf("Request text was not redacted properly: %q", redactedText)
	}

	// Fake an LLM response containing the redacted details
	resp := &model.LLMResponse{
		Content: &genai.Content{
			Parts: []*genai.Part{
				{
					Text: "I will forward your inquiry to " + redactedText + " right away.",
				},
			},
		},
	}

	// Run AfterModelCallback to unredact
	_, err = p.AfterModelCallback()(ctx, resp, nil)
	if err != nil {
		t.Fatalf("AfterModelCallback failed: %v", err)
	}

	unredactedText := resp.Content.Parts[0].Text
	expectedSub := "help.desk@support.com"
	if !strings.Contains(unredactedText, expectedSub) {
		t.Errorf("Response text was not restored properly.\nExpected substring: %q\nGot:                %q", expectedSub, unredactedText)
	}
}
