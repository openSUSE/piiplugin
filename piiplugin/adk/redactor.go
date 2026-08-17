// Package adk contains the Google ADK plugin bindings for the PII filters in
// the filter sub-packages. It is the only place in this module that depends on
// the ADK: the filter engines themselves (and the pure piiplugin composite) are
// free of any ADK import, so they can be used without it.
package adk

import (
	"strings"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/plugin"
	"google.golang.org/adk/v2/tool"

	"github.com/openSUSE/piiplugin/filter"
)

// Redactor is what a PII filter engine must provide to be wrapped by the ADK
// adapter. All the filter engines in this module (username, host and eMail)
// satisfy it.
type Redactor interface {
	// Redact redacts text in the direction leaving the machine. fullInput is the
	// whole payload the replacement must not collide with.
	Redact(text, fullInput string) string
	// Unredact restores the original values in text.
	Unredact(text string) string
}

// redactorPlugin adapts a single Redactor to the six ADK plugin callbacks. The
// same logic is shared by the username, host and eMail filters.
type redactorPlugin struct {
	redact   func(text, fullInput string) string
	unredact func(text string) string
}

// newRedactorPlugin wraps r in an ADK plugin under the given name.
func newRedactorPlugin(name string, r Redactor) (*plugin.Plugin, error) {
	p := &redactorPlugin{redact: r.Redact, unredact: r.Unredact}
	return plugin.New(plugin.Config{
		Name:                 name,
		BeforeModelCallback:  p.BeforeModelCallback,
		AfterModelCallback:   p.AfterModelCallback,
		OnModelErrorCallback: p.OnModelErrorCallback,
		BeforeToolCallback:   p.BeforeToolCallback,
		AfterToolCallback:    p.AfterToolCallback,
		OnToolErrorCallback:  p.OnToolErrorCallback,
	})
}

// fullInputText gathers all text from an LLM request so that generated
// replacements can be checked against the whole input.
func fullInputText(req *model.LLMRequest) string {
	if req == nil {
		return ""
	}
	var sb strings.Builder
	for _, content := range req.Contents {
		if content == nil {
			continue
		}
		for _, part := range content.Parts {
			if part == nil {
				continue
			}
			sb.WriteString(part.Text)
			sb.WriteString(" ")
		}
	}
	return sb.String()
}

// BeforeModelCallback redacts the text of every request part.
func (p *redactorPlugin) BeforeModelCallback(ctx agent.Context, req *model.LLMRequest) (*model.LLMResponse, error) {
	if req == nil {
		return nil, nil
	}
	fullInput := fullInputText(req)
	for _, content := range req.Contents {
		if content == nil {
			continue
		}
		for _, part := range content.Parts {
			if part == nil || part.Text == "" {
				continue
			}
			part.Text = p.redact(part.Text, fullInput)
		}
	}
	return nil, nil
}

// AfterModelCallback restores the original values in the response.
func (p *redactorPlugin) AfterModelCallback(ctx agent.Context, resp *model.LLMResponse, err error) (*model.LLMResponse, error) {
	if resp == nil || resp.Content == nil {
		return nil, nil
	}
	for _, part := range resp.Content.Parts {
		if part == nil || part.Text == "" {
			continue
		}
		part.Text = p.unredact(part.Text)
	}
	return nil, nil
}

// OnModelErrorCallback is a pass-through for model errors.
func (p *redactorPlugin) OnModelErrorCallback(ctx agent.Context, req *model.LLMRequest, err error) (*model.LLMResponse, error) {
	return nil, nil
}

// BeforeToolCallback restores the original values in the tool arguments, so the
// tool runs against the real system.
func (p *redactorPlugin) BeforeToolCallback(ctx agent.Context, t tool.Tool, args map[string]any) (map[string]any, error) {
	filter.MapToolValues(args, p.unredact)
	return nil, nil
}

// AfterToolCallback redacts the tool result and the arguments again, since both
// stay in the session and are resent with every following request.
func (p *redactorPlugin) AfterToolCallback(ctx agent.Context, t tool.Tool, args, result map[string]any, err error) (map[string]any, error) {
	fullInput := filter.ToolValuesText(args) + filter.ToolValuesText(result)
	redact := func(text string) string { return p.redact(text, fullInput) }
	filter.MapToolValues(args, redact)
	if result != nil {
		filter.MapToolValues(result, redact)
	}
	return nil, nil
}

// OnToolErrorCallback redacts the message of a failed tool call into a
// {"error": ...} result.
func (p *redactorPlugin) OnToolErrorCallback(ctx agent.Context, t tool.Tool, args map[string]any, err error) (map[string]any, error) {
	if err == nil {
		return nil, nil
	}
	message := err.Error()
	return map[string]any{"error": p.redact(message, message)}, nil
}
