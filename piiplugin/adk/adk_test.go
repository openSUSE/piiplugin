package adk

import (
	"errors"
	"strings"
	"testing"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"

	"github.com/openSUSE/piiplugin/filter"
	filterhost "github.com/openSUSE/piiplugin/filter/host"
	filterusername "github.com/openSUSE/piiplugin/filter/username"
	"github.com/openSUSE/piiplugin/piiplugin"
)

// mockContext satisfies the agent.Context interface by embedding the
// interface itself (agent.Context is itself the context type used by the
// callbacks).
type mockContext struct {
	agent.Context
}

// newMockUsers is a getpasswd function that returns a fixed user database.
func mockUsers() func() ([]string, error) {
	return func() ([]string, error) {
		return []string{
			"jdoe:x:1001:1001:John Doe,Room 101,,:/home/jdoe:/bin/bash",
			"alice:x:1002:1002:Alice,,::",
		}, nil
	}
}

func TestUsernamePlugin_RedactAndUnredact(t *testing.T) {
	filter.UseMock = true
	defer func() { filter.UseMock = false }()

	plug, err := NewUsernamePlugin(
		filterusername.WithGetpasswdFunc(mockUsers()),
	)
	if err != nil {
		t.Fatalf("Failed to create UsernamePlugin: %v", err)
	}

	inputText := "Hello John, your username is jdoe. Is Alice here? Do not replace Johnathan or DoeMaster."
	req := &model.LLMRequest{
		Contents: []*genai.Content{{Parts: []*genai.Part{{Text: inputText}}}},
	}

	ctx := &mockContext{}
	if _, err := plug.BeforeModelCallback()(ctx, req); err != nil {
		t.Fatalf("BeforeModelCallback failed: %v", err)
	}

	redacted := req.Contents[0].Parts[0].Text
	if !strings.Contains(redacted, "nhoJ") || !strings.Contains(redacted, "eodj") || !strings.Contains(redacted, "ecilA") {
		t.Errorf("Expected names to be redacted, got: %q", redacted)
	}
	if strings.Contains(redacted, "nhoJohnathan") || !strings.Contains(redacted, "Johnathan") {
		t.Errorf("Johnathan should not be partially redacted: %q", redacted)
	}
	if strings.Contains(redacted, "eoD") || !strings.Contains(redacted, "DoeMaster") {
		t.Errorf("DoeMaster should not be partially redacted: %q", redacted)
	}

	resp := &model.LLMResponse{Content: &genai.Content{Parts: []*genai.Part{{Text: redacted}}}}
	if _, err := plug.AfterModelCallback()(ctx, resp, nil); err != nil {
		t.Fatalf("AfterModelCallback failed: %v", err)
	}
	if got := resp.Content.Parts[0].Text; got != inputText {
		t.Errorf("Unredacting failed.\nExpected: %q\nGot:      %q", inputText, got)
	}
}

func TestUsernamePlugin_ToolCallbacks(t *testing.T) {
	filter.UseMock = true
	defer func() { filter.UseMock = false }()

	plug, err := NewUsernamePlugin(filterusername.WithGetpasswdFunc(mockUsers()))
	if err != nil {
		t.Fatalf("Failed to create plugin: %v", err)
	}
	ctx := &mockContext{}
	args := map[string]any{}

	result := map[string]any{
		"processes": []any{
			map[string]any{
				"pid":      1234,
				"username": "jdoe",
				"cmdline":  "/home/jdoe/bin/backup",
			},
		},
	}
	if _, err := plug.AfterToolCallback()(ctx, nil, args, result, nil); err != nil {
		t.Fatalf("AfterToolCallback failed: %v", err)
	}

	proc := result["processes"].([]any)[0].(map[string]any)
	if proc["username"] != "eodj" {
		t.Errorf("Expected username to be redacted to eodj, got: %v", proc["username"])
	}
	if proc["cmdline"] != "/home/eodj/bin/backup" {
		t.Errorf("Expected cmdline to be redacted, got: %v", proc["cmdline"])
	}
	if proc["pid"] != 1234 {
		t.Errorf("Non string values must be kept, got: %v", proc["pid"])
	}

	nextArgs := map[string]any{"filter_name": "eodj"}
	if _, err := plug.BeforeToolCallback()(ctx, nil, nextArgs); err != nil {
		t.Fatalf("BeforeToolCallback failed: %v", err)
	}
	if nextArgs["filter_name"] != "jdoe" {
		t.Errorf("Expected tool argument to be restored to jdoe, got: %v", nextArgs["filter_name"])
	}

	if _, err := plug.AfterToolCallback()(ctx, nil, nextArgs, map[string]any{}, nil); err != nil {
		t.Fatalf("AfterToolCallback failed: %v", err)
	}
	if nextArgs["filter_name"] != "eodj" {
		t.Errorf("Expected tool argument to be redacted again, got: %v", nextArgs["filter_name"])
	}

	errResult, err := plug.OnToolErrorCallback()(ctx, nil, args, errors.New("ps failed for jdoe"))
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

func TestEmailPlugin_Callbacks(t *testing.T) {
	filter.UseMock = true
	defer func() { filter.UseMock = false }()

	plug, err := NewEmailPlugin()
	if err != nil {
		t.Fatalf("Failed to create EmailPlugin: %v", err)
	}

	req := &model.LLMRequest{
		Contents: []*genai.Content{{Parts: []*genai.Part{{Text: "Help desk email is help.desk@support.com. Please write to them."}}}},
	}
	ctx := &mockContext{}
	if _, err := plug.BeforeModelCallback()(ctx, req); err != nil {
		t.Fatalf("BeforeModelCallback failed: %v", err)
	}
	redacted := req.Contents[0].Parts[0].Text
	if strings.Contains(redacted, "help.desk") || strings.Contains(redacted, "support.com") {
		t.Errorf("Request text was not redacted properly: %q", redacted)
	}

	resp := &model.LLMResponse{Content: &genai.Content{Parts: []*genai.Part{{Text: "I will forward your inquiry to " + redacted + " right away."}}}}
	if _, err := plug.AfterModelCallback()(ctx, resp, nil); err != nil {
		t.Fatalf("AfterModelCallback failed: %v", err)
	}
	unredacted := resp.Content.Parts[0].Text
	if !strings.Contains(unredacted, "help.desk@support.com") {
		t.Errorf("Response text was not restored properly.\nExpected substring: %q\nGot: %q", "help.desk@support.com", unredacted)
	}
}

func TestHostPlugin_RedactAndUnredact(t *testing.T) {
	filter.UseMock = true
	defer func() { filter.UseMock = false }()

	plug, err := NewHostPlugin(
		filterhost.WithDomain("myoffice.internal"),
		filterhost.WithDNSServer("192.168.1.1"),
		filterhost.WithLookupFunc(func(string) ([]string, error) {
			return []string{"mailserver.myoffice.internal."}, nil
		}),
	)
	if err != nil {
		t.Fatalf("Failed to create HostPlugin: %v", err)
	}

	ctx := &mockContext{}
	req := &model.LLMRequest{
		Contents: []*genai.Content{{Parts: []*genai.Part{{Text: "Connect to mailserver.myoffice.internal or gateway."}}}},
	}
	if _, err := plug.BeforeModelCallback()(ctx, req); err != nil {
		t.Fatalf("BeforeModelCallback failed: %v", err)
	}
	redacted := req.Contents[0].Parts[0].Text
	if strings.Contains(redacted, "mailserver.myoffice.internal") {
		t.Errorf("Expected the host to be redacted, got: %q", redacted)
	}
}

func TestNewPiiPlugin_Integration(t *testing.T) {
	filter.UseMock = true
	defer func() { filter.UseMock = false }()

	plug, err := NewPiiPlugin(
		piiplugin.WithUsernameSource(filterusername.SourceGetent),
	)
	if err != nil {
		t.Fatalf("Failed to create PiiPlugin: %v", err)
	}

	ctx := &mockContext{}
	text := "Contact gecko@earth.example.com as geeko."
	req := &model.LLMRequest{
		Contents: []*genai.Content{{Parts: []*genai.Part{{Text: text}}}},
	}

	if _, err := plug.BeforeModelCallback()(ctx, req); err != nil {
		t.Fatalf("BeforeModelCallback failed: %v", err)
	}
	redacted := req.Contents[0].Parts[0].Text
	if strings.Contains(redacted, "gecko@earth.example.com") {
		t.Errorf("Expected the email to be redacted, got: %q", redacted)
	}

	resp := &model.LLMResponse{Content: &genai.Content{Parts: []*genai.Part{{Text: "reached " + redacted}}}}
	if _, err := plug.AfterModelCallback()(ctx, resp, nil); err != nil {
		t.Fatalf("AfterModelCallback failed: %v", err)
	}
	unredacted := resp.Content.Parts[0].Text
	if !strings.Contains(unredacted, "gecko@earth.example.com") {
		t.Errorf("Expected the address to be restored, got: %q", unredacted)
	}
}
