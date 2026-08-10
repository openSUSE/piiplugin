package filterusername

import (
	"errors"
	"strings"
	"testing"

	"github.com/openSUSE/piirplug/filter"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"
)

// mockContext implements agent.Context for testing callbacks
type mockContext struct {
	agent.Context
}

func newReplMap() *map[string]string {
	m := make(map[string]string)
	return &m
}

func TestNewUsernamePlugin_Defaults(t *testing.T) {
	plugin, err := NewUsernamePlugin()
	if err != nil {
		t.Fatalf("Failed to create UsernamePlugin: %v", err)
	}
	if plugin == nil {
		t.Fatal("Plugin should not be nil")
	}
}

func TestUsernamePlugin_RedactAndUnredact_CustomGetpasswdFunc(t *testing.T) {
	// Let's enable mock replacements (reverses original names) so tests are deterministic.
	filter.UseMock = true
	defer func() {
		filter.UseMock = false
	}()

	replacements := newReplMap()
	getpasswdMock := func() ([]string, error) {
		return []string{
			"jdoe:x:1001:1001:John Doe,Room 101,,:/home/jdoe:/bin/bash",
			"alice:x:1002:1002:Alice InWonderland,,:/home/alice:/bin/bash",
			"system:x:500:500:System User,,:/home/system:/bin/bash", // should be ignored (UID < 1000)
		}, nil
	}

	plugin, err := NewUsernamePlugin(
		WithReplacement(replacements),
		WithGetpasswdFunc(getpasswdMock),
	)
	if err != nil {
		t.Fatalf("Failed to create UsernamePlugin: %v", err)
	}

	inputText := "Hello John, your username is jdoe. Is Alice here? But do not replace Johnathan or DoeMaster. And System must not be replaced."

	req := &model.LLMRequest{
		Contents: []*genai.Content{
			{
				Parts: []*genai.Part{
					{Text: inputText},
				},
			},
		},
	}

	ctx := &mockContext{}
	_, err = plugin.BeforeModelCallback()(ctx, req)
	if err != nil {
		t.Fatalf("BeforeModelCallback failed: %v", err)
	}

	redacted := req.Contents[0].Parts[0].Text

	// John -> nhoJ, jdoe -> eodj, Alice -> ecilA
	if !strings.Contains(redacted, "nhoJ") {
		t.Errorf("Expected 'John' to be replaced with 'nhoJ', got: %q", redacted)
	}
	if !strings.Contains(redacted, "eodj") {
		t.Errorf("Expected 'jdoe' to be replaced with 'eodj', got: %q", redacted)
	}
	if !strings.Contains(redacted, "ecilA") {
		t.Errorf("Expected 'Alice' to be replaced with 'ecilA' (case-insensitive), got: %q", redacted)
	}

	// System should NOT be replaced (UID < 1000)
	if strings.Contains(redacted, "metsyS") || !strings.Contains(redacted, "System") {
		t.Errorf("System user (UID < 1000) must NOT be redacted: %q", redacted)
	}

	// Make sure word boundaries are respected
	if strings.Contains(redacted, "nhoJohnathan") || strings.Contains(redacted, "Johnathan") == false {
		t.Errorf("Johnathan should NOT have been partially redacted: %q", redacted)
	}
	if strings.Contains(redacted, "eoD") || strings.Contains(redacted, "DoeMaster") == false {
		t.Errorf("DoeMaster should NOT have been partially redacted: %q", redacted)
	}

	// Create response to restore original names
	resp := &model.LLMResponse{
		Content: &genai.Content{
			Parts: []*genai.Part{
				{Text: redacted},
			},
		},
	}

	_, err = plugin.AfterModelCallback()(ctx, resp, nil)
	if err != nil {
		t.Fatalf("AfterModelCallback failed: %v", err)
	}

	unredacted := resp.Content.Parts[0].Text
	if unredacted != inputText {
		t.Errorf("Unredacting failed.\nExpected: %q\nGot:      %q", inputText, unredacted)
	}
}

func TestUsernamePlugin_Callbacks(t *testing.T) {
	filter.UseMock = true
	defer func() {
		filter.UseMock = false
	}()

	replMap := newReplMap()
	getpasswdMock := func() ([]string, error) {
		return []string{
			"jdoe:x:1001:1001:John Doe,Room 101,,:/home/jdoe:/bin/bash",
		}, nil
	}

	plugin, err := NewUsernamePlugin(
		WithReplacement(replMap),
		WithGetpasswdFunc(getpasswdMock),
	)
	if err != nil {
		t.Fatalf("Failed to create plugin: %v", err)
	}

	// Construct LLMRequest
	req := &model.LLMRequest{
		Contents: []*genai.Content{
			{
				Parts: []*genai.Part{
					{Text: "Welcome back John!"},
				},
			},
		},
	}

	ctx := &mockContext{}
	_, err = plugin.BeforeModelCallback()(ctx, req)
	if err != nil {
		t.Fatalf("BeforeModelCallback failed: %v", err)
	}

	redactedText := req.Contents[0].Parts[0].Text
	if !strings.Contains(redactedText, "nhoJ") {
		t.Errorf("BeforeModelCallback did not redact John: %q", redactedText)
	}

	// Construct LLMResponse
	resp := &model.LLMResponse{
		Content: &genai.Content{
			Parts: []*genai.Part{
				{Text: "Response to nhoJ"},
			},
		},
	}

	_, err = plugin.AfterModelCallback()(ctx, resp, nil)
	if err != nil {
		t.Fatalf("AfterModelCallback failed: %v", err)
	}

	unredactedText := resp.Content.Parts[0].Text
	if !strings.Contains(unredactedText, "John") {
		t.Errorf("AfterModelCallback did not restore John: %q", unredactedText)
	}
}

// TestUsernamePlugin_ToolCallbacks checks the tool round trip: the arguments
// the model sends are restored to the real names before the tool runs and the
// result of the tool is redacted before it goes back to the model.
func TestUsernamePlugin_ToolCallbacks(t *testing.T) {
	filter.UseMock = true
	defer func() {
		filter.UseMock = false
	}()

	replMap := newReplMap()
	getpasswdMock := func() ([]string, error) {
		return []string{
			"jdoe:x:1001:1001:John Doe,Room 101,,:/home/jdoe:/bin/bash",
		}, nil
	}

	plugin, err := NewUsernamePlugin(
		WithReplacement(replMap),
		WithGetpasswdFunc(getpasswdMock),
	)
	if err != nil {
		t.Fatalf("Failed to create plugin: %v", err)
	}

	ctx := &mockContext{}
	args := map[string]any{}

	// A get_processes style result, the payload is a map and not text.
	result := map[string]any{
		"processes": []any{
			map[string]any{
				"pid":      1234,
				"username": "jdoe",
				"name":     "bash",
				"cmdline":  "/home/jdoe/bin/backup",
			},
		},
	}
	if _, err := plugin.AfterToolCallback()(ctx, nil, args, result, nil); err != nil {
		t.Fatalf("AfterToolCallback failed: %v", err)
	}

	proc := result["processes"].([]any)[0].(map[string]any)
	if proc["username"] != "eodj" {
		t.Errorf("Expected the username in the tool result to be redacted, got: %v", proc["username"])
	}
	if proc["cmdline"] != "/home/eodj/bin/backup" {
		t.Errorf("Expected the username in the command line to be redacted, got: %v", proc["cmdline"])
	}
	if proc["pid"] != 1234 {
		t.Errorf("Non string values of the result must be kept, got: %v", proc["pid"])
	}

	// The model has only seen the replacement, so a follow up call asks the
	// tool for "eodj", which has to be turned back into the real name.
	nextArgs := map[string]any{"filter_name": "eodj"}
	if _, err := plugin.BeforeToolCallback()(ctx, nil, nextArgs); err != nil {
		t.Fatalf("BeforeToolCallback failed: %v", err)
	}
	if nextArgs["filter_name"] != "jdoe" {
		t.Errorf("Expected the tool argument to be restored to 'jdoe', got: %v", nextArgs["filter_name"])
	}

	// The arguments belong to the function call kept in the session, so they
	// have to carry the replacement again once the tool has run.
	if _, err := plugin.AfterToolCallback()(ctx, nil, nextArgs, map[string]any{}, nil); err != nil {
		t.Fatalf("AfterToolCallback failed: %v", err)
	}
	if nextArgs["filter_name"] != "eodj" {
		t.Errorf("Expected the tool argument to be redacted again, got: %v", nextArgs["filter_name"])
	}

	// A failing tool must not leak the name through its error message either.
	errResult, err := plugin.OnToolErrorCallback()(ctx, nil, args, errors.New("ps failed for jdoe"))
	if err != nil {
		t.Fatalf("OnToolErrorCallback failed: %v", err)
	}
	message, ok := errResult["error"].(string)
	if !ok {
		t.Fatalf("Expected an error message in the result, got: %v", errResult)
	}
	if strings.Contains(message, "jdoe") || !strings.Contains(message, "eodj") {
		t.Errorf("Expected the error message to be redacted, got: %q", message)
	}
}

func TestUniqueNames(t *testing.T) {
	input := []string{"John", "john", "DOE", "Doe", "alice"}
	expected := []string{"John", "DOE", "alice"}

	output := uniqueNames(input)
	if len(output) != len(expected) {
		t.Fatalf("Expected unique names length %d, got %d", len(expected), len(output))
	}
	for i, v := range output {
		if strings.ToLower(v) != strings.ToLower(expected[i]) {
			t.Errorf("At index %d, expected %q, got %q", i, expected[i], v)
		}
	}
}
