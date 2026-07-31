package piiplugin

import (
	"strings"
	"testing"

	"github.com/openSUSE/piiplug/filter"
	filteremail "github.com/openSUSE/piiplug/filter/email"
	filterusername "github.com/openSUSE/piiplug/filter/username"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"
)

type mockContext struct {
	agent.Context
}

func TestPiiPlugin_Integration(t *testing.T) {
	filter.UseMock = true
	defer func() {
		filter.UseMock = false
	}()

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

	// Create request containing both an email and an integrated username
	req := &model.LLMRequest{
		Contents: []*genai.Content{
			{
				Parts: []*genai.Part{
					{Text: "Send email to alice@company.com and greet Bob!"},
				},
			},
		},
	}

	ctx := &mockContext{}
	_, err = p.BeforeModelCallback(ctx, req)
	if err != nil {
		t.Fatalf("BeforeModelCallback failed: %v", err)
	}

	redactedText := req.Contents[0].Parts[0].Text

	// Verify both got redacted
	if strings.Contains(redactedText, "alice@company.com") || strings.Contains(redactedText, "Bob") {
		t.Errorf("Expected redaction of both email and username, got: %q", redactedText)
	}

	// Verify mock reversal
	if !strings.Contains(redactedText, "boB") {
		t.Errorf("Expected Bob to be redacted to boB, got: %q", redactedText)
	}

	// Create response to restore original names
	resp := &model.LLMResponse{
		Content: &genai.Content{
			Parts: []*genai.Part{
				{Text: "Message for boB and ecila@ynapmoc.com"},
			},
		},
	}

	_, err = p.AfterModelCallback(ctx, resp, nil)
	if err != nil {
		t.Fatalf("AfterModelCallback failed: %v", err)
	}

	unredactedText := resp.Content.Parts[0].Text
	if !strings.Contains(unredactedText, "Bob") || !strings.Contains(unredactedText, "alice@company.com") {
		t.Errorf("Expected full unredaction, got: %q", unredactedText)
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
}
