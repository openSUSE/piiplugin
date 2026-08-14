package piiplugin

import (
	"errors"
	"strings"
	"testing"

	"github.com/openSUSE/piiplugin/filter"
	filteremail "github.com/openSUSE/piiplugin/filter/email"
	filterhost "github.com/openSUSE/piiplugin/filter/host"
	filterusername "github.com/openSUSE/piiplugin/filter/username"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"
)

type mockContext struct {
	agent.Context
}

// newTestPiiPlugin builds a PiiPlugin whose three filters share one replacement
// table and know a fixed set of users and hosts.
func newTestPiiPlugin(t *testing.T) *PiiPlugin {
	t.Helper()

	p := &PiiPlugin{}
	replacements := make(map[string]string)

	emailPlg, err := filteremail.NewEmailPlugin(
		filteremail.WithReplacement(&replacements),
	)
	if err != nil {
		t.Fatalf("Failed to create email plugin: %v", err)
	}
	p.eMailPlugin = emailPlg

	userPlg, err := filterusername.NewUsernamePlugin(
		filterusername.WithReplacement(&replacements),
		filterusername.WithGetpasswdFunc(func() ([]string, error) {
			return []string{
				"alice:x:1001:1001:Alice,,::",
				"bob:x:1002:1002:Bob,,::",
			}, nil
		}),
	)
	if err != nil {
		t.Fatalf("Failed to create username plugin: %v", err)
	}
	p.userNamePlugin = userPlg

	hostPlg, err := filterhost.NewHostPlugin(
		filterhost.WithReplacement(&replacements),
		filterhost.WithDomain("myoffice.internal"),
		filterhost.WithDNSServer("192.168.1.1"),
		filterhost.WithLookupFunc(func(domain string) ([]string, error) {
			return []string{"mailserver.myoffice.internal."}, nil
		}),
	)
	if err != nil {
		t.Fatalf("Failed to create host plugin: %v", err)
	}
	p.hostPlugin = hostPlg

	return p
}

func TestPiiPlugin_Integration(t *testing.T) {
	filter.UseMock = true
	defer func() {
		filter.UseMock = false
	}()

	p := newTestPiiPlugin(t)

	// Create request containing email, username, and hostname
	req := &model.LLMRequest{
		Contents: []*genai.Content{
			{
				Parts: []*genai.Part{
					{Text: "Send email to alice@company.com, greet Bob, and check mailserver.myoffice.internal!"},
				},
			},
		},
	}

	ctx := &mockContext{}
	_, err := p.BeforeModelCallback(ctx, req)
	if err != nil {
		t.Fatalf("BeforeModelCallback failed: %v", err)
	}

	redactedText := req.Contents[0].Parts[0].Text

	// Verify all got redacted
	if strings.Contains(redactedText, "alice@company.com") || strings.Contains(redactedText, "Bob") || strings.Contains(redactedText, "mailserver.myoffice.internal") {
		t.Errorf("Expected redaction of email, username, and host, got: %q", redactedText)
	}

	// Verify mock reversal for Bob and Host
	if !strings.Contains(redactedText, "boB") {
		t.Errorf("Expected Bob to be redacted to boB, got: %q", redactedText)
	}
	if !strings.Contains(redactedText, "lanretni.eciffoym.revresliam") {
		t.Errorf("Expected mailserver.myoffice.internal to be redacted to lanretni.eciffoym.revresliam, got: %q", redactedText)
	}

	// Create response to restore original names
	resp := &model.LLMResponse{
		Content: &genai.Content{
			Parts: []*genai.Part{
				{Text: "Message for boB, ecila@ynapmoc.com, and lanretni.eciffoym.revresliam"},
			},
		},
	}

	_, err = p.AfterModelCallback(ctx, resp, nil)
	if err != nil {
		t.Fatalf("AfterModelCallback failed: %v", err)
	}

	unredactedText := resp.Content.Parts[0].Text
	// Debug: print the unredacted text for inspection
	t.Logf("Unredacted text: %q", unredactedText)
	if !strings.Contains(unredactedText, "Bob") || !strings.Contains(unredactedText, "alice@company.com") || !strings.Contains(unredactedText, "mailserver.myoffice.internal") {
		t.Errorf("Expected full unredaction, got: %q", unredactedText)
	}
}

// TestPiiPlugin_ToolCallbacks verifies that a tool result, which reaches the
// model as a FunctionResponse and not as text, is redacted by all filters and
// that the arguments of the next tool call are restored again.
func TestPiiPlugin_ToolCallbacks(t *testing.T) {
	filter.UseMock = true
	defer func() {
		filter.UseMock = false
	}()

	p := newTestPiiPlugin(t)
	ctx := &mockContext{}
	args := map[string]any{}

	result := map[string]any{
		"entries": []any{
			map[string]any{
				"uid":     1002,
				"user":    "Bob",
				"contact": "alice@company.com",
				"host":    "mailserver.myoffice.internal",
			},
		},
	}

	if _, err := p.AfterToolCallback(ctx, nil, args, result, nil); err != nil {
		t.Fatalf("AfterToolCallback failed: %v", err)
	}

	entry := result["entries"].([]any)[0].(map[string]any)
	for field, original := range map[string]string{
		"user":    "Bob",
		"contact": "alice@company.com",
		"host":    "mailserver.myoffice.internal",
	} {
		if got := entry[field].(string); strings.Contains(got, original) {
			t.Errorf("Expected %q to be redacted in field %q, got: %q", original, field, got)
		}
	}
	if entry["user"] != "boB" {
		t.Errorf("Expected Bob to be redacted to boB, got: %v", entry["user"])
	}
	if entry["host"] != "lanretni.eciffoym.revresliam" {
		t.Errorf("Expected the host to be redacted, got: %v", entry["host"])
	}
	if entry["uid"] != 1002 {
		t.Errorf("Non string values of the result must be kept, got: %v", entry["uid"])
	}

	// The model answers with the replacements it has seen, so the arguments of
	// a follow up tool call have to be restored before the tool runs.
	nextArgs := map[string]any{
		"user": "boB",
		"host": "lanretni.eciffoym.revresliam",
	}
	if _, err := p.BeforeToolCallback(ctx, nil, nextArgs); err != nil {
		t.Fatalf("BeforeToolCallback failed: %v", err)
	}
	if nextArgs["user"] != "Bob" || nextArgs["host"] != "mailserver.myoffice.internal" {
		t.Errorf("Expected the tool arguments to be restored, got: %v", nextArgs)
	}

	// The arguments belong to the function call kept in the session, so they
	// have to carry the replacements again once the tool has run.
	if _, err := p.AfterToolCallback(ctx, nil, nextArgs, map[string]any{}, nil); err != nil {
		t.Fatalf("AfterToolCallback failed: %v", err)
	}
	if nextArgs["user"] != "boB" || nextArgs["host"] != "lanretni.eciffoym.revresliam" {
		t.Errorf("Expected the tool arguments to be redacted again, got: %v", nextArgs)
	}
}

// TestPiiPlugin_OnToolErrorCallback makes sure a failing tool does not leak the
// data through its error message.
func TestPiiPlugin_OnToolErrorCallback(t *testing.T) {
	filter.UseMock = true
	defer func() {
		filter.UseMock = false
	}()

	p := newTestPiiPlugin(t)

	result, err := p.OnToolErrorCallback(&mockContext{}, nil,
		map[string]any{}, errors.New("cannot reach mailserver.myoffice.internal as Bob"))
	if err != nil {
		t.Fatalf("OnToolErrorCallback failed: %v", err)
	}

	message, ok := result["error"].(string)
	if !ok {
		t.Fatalf("Expected an error message in the result, got: %v", result)
	}
	if strings.Contains(message, "Bob") || strings.Contains(message, "mailserver.myoffice.internal") {
		t.Errorf("Expected the error message to be redacted, got: %q", message)
	}
}

func TestPiiPlugin_Options(t *testing.T) {
	// 1. Without Email option
	p1 := &PiiPlugin{}
	WithoutEmail()(p1)
	if !p1.noEMail {
		t.Error("noEMail should be true when WithoutEmail() is used")
	}

	// 2. Without Username option
	p2 := &PiiPlugin{}
	WithoutUsername()(p2)
	if !p2.noUserName {
		t.Error("noUserName should be true when WithoutUsername() is used")
	}

	// 3. Without Host option
	p3 := &PiiPlugin{}
	WithoutHost()(p3)
	if !p3.noHost {
		t.Error("noHost should be true when WithoutHost() is used")
	}
}

func TestPiiFilter(t *testing.T) {
	filter.UseMock = true
	defer func() {
		filter.UseMock = false
	}()

	f := NewPiiFilter(WithoutUsername(), WithoutHost())

	inputText := "Send email to alice@company.com or bob@office.org."
	redacted := f.Redact(inputText)

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

	// Create filter with only email enabled, similar to existing TestPiiFilter
	f := NewPiiFilter(WithoutUsername(), WithoutHost())

	inputText := "Contact john.doe@company.com or jane.roe@example.org for help."
	redacted := f.Redact(inputText)

	// Verify redaction happened - emails should be replaced with tokens
	if strings.Contains(redacted, "john.doe") || strings.Contains(redacted, "company.com") {
		t.Errorf("Expected email to be redacted, got: %q", redacted)
	}
	if strings.Contains(redacted, "jane.roe") || strings.Contains(redacted, "example.org") {
		t.Errorf("Expected email to be redacted, got: %q", redacted)
	}

	// Verify Replacements map is accessible and populated
	if f.Replacements == nil {
		t.Fatal("Replacements map should not be nil")
	}

	replacements := *f.Replacements
	if len(replacements) == 0 {
		t.Error("Expected at least one replacement to be generated")
	}

	// Check that the redacted tokens exist in the replacements map
	for redactedToken, originalValue := range replacements {
		if redactedToken == "" {
			t.Error("Redacted token should not be empty string")
		}
		if originalValue == "" {
			t.Error("Original value should not be empty string")
		}
		// Verify the redacted token appears in the redacted text
		if !strings.Contains(redacted, redactedToken) {
			t.Errorf("Expected redacted token %q to appear in redacted text %q", redactedToken, redacted)
		}
		// Verify original value does NOT appear in redacted text
		if strings.Contains(redacted, originalValue) {
			t.Errorf("Original value %q should not appear in redacted text %q", originalValue, redacted)
		}
	}

	// Test Unredact uses the same map - verify round-trip works
	unredacted := f.Unredact(redacted)
	if unredacted != inputText {
		t.Errorf("Expected round-trip to restore original text\nGot: %q\nWant:%q", unredacted, inputText)
	}
}
