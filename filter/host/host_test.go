package filterhost

import (
	"strings"
	"testing"

	"github.com/openSUSE/piiplug/filter"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"
)

type mockContext struct {
	agent.Context
}

func newReplMap() *map[string]string {
	m := make(map[string]string)
	return &m
}

func TestNewHostPlugin_Defaults(t *testing.T) {
	plugin, err := NewHostPlugin()
	if err != nil {
		t.Fatalf("Failed to create HostPlugin: %v", err)
	}
	if plugin == nil {
		t.Fatal("Plugin should not be nil")
	}
}

func TestHostPlugin_RedactAndUnredact(t *testing.T) {
	filter.UseMock = true
	defer func() {
		filter.UseMock = false
	}()

	replacements := newReplMap()
	lookupMock := func(domain string) ([]string, error) {
		return []string{
			"gateway.myoffice.internal.",
			"mailserver.myoffice.internal.",
			"workstation42.myoffice.internal.",
		}, nil
	}

	plugin, err := NewHostPlugin(
		WithReplacement(replacements),
		WithDomain("myoffice.internal"),
		WithDNSServer("192.168.1.1"),
		WithLookupFunc(lookupMock),
	)
	if err != nil {
		t.Fatalf("Failed to create HostPlugin: %v", err)
	}

	inputText := "Connect to mailserver.myoffice.internal or gateway. We also have workstation42 and our internal domain is myoffice.internal."
	
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

	// mailserver.myoffice.internal -> lanretni.eciffoym.revresliam
	// gateway -> yawetag
	// workstation42 -> 24noitatskrow
	// myoffice.internal -> lanretni.eciffoym
	if !strings.Contains(redacted, "lanretni.eciffoym.revresliam") {
		t.Errorf("Expected 'mailserver.myoffice.internal' to be replaced, got: %q", redacted)
	}
	if !strings.Contains(redacted, "yawetag") {
		t.Errorf("Expected 'gateway' to be replaced, got: %q", redacted)
	}
	if !strings.Contains(redacted, "24noitatskrow") {
		t.Errorf("Expected 'workstation42' to be replaced, got: %q", redacted)
	}
	if !strings.Contains(redacted, "lanretni.eciffoym") {
		t.Errorf("Expected 'myoffice.internal' to be replaced, got: %q", redacted)
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

func TestCleanHostAndDomainNames(t *testing.T) {
	input := []string{
		"myhost.mycompany.local",
		"192.168.1.1", // IP address - should be ignored
		"a",           // too short - should be ignored
	}
	expected := []string{
		"myhost.mycompany.local",
		"myhost",
		"mycompany.local",
	}

	output := cleanHostAndDomainNames(input)
	if len(output) != len(expected) {
		t.Fatalf("Expected %d clean names, got %d: %v", len(expected), len(output), output)
	}
	for i, v := range output {
		if v != expected[i] {
			t.Errorf("At index %d, expected %q, got %q", i, expected[i], v)
		}
	}
}
