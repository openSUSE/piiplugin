package filter

import (
	"regexp"
	"sort"
	"strings"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/tool"
)

// UniqueNamesPlugin manages a shared replacement table and performs
// redaction and unredaction on a list of unique names/strings.
type UniqueNamesPlugin struct {
	Replacements *map[string]string
	Regex        *regexp.Regexp
}

// NewUniqueNamesPlugin creates a new UniqueNamesPlugin with the given options and name list.
func NewUniqueNamesPlugin(replacements *map[string]string, names []string) (*UniqueNamesPlugin, error) {
	p := &UniqueNamesPlugin{
		Replacements: replacements,
	}
	if p.Replacements == nil {
		m := make(map[string]string)
		p.Replacements = &m
	}

	if err := p.InitRegex(names); err != nil {
		return nil, err
	}

	return p, nil
}

// InitRegex builds the regular expression from the given list of names.
func (p *UniqueNamesPlugin) InitRegex(names []string) error {
	if len(names) == 0 {
		p.Regex = nil
		return nil
	}
	escaped := make([]string, len(names))
	for i, name := range names {
		escaped[i] = regexp.QuoteMeta(name)
	}
	// Sort by length descending so that longer names match before their prefixes
	sort.Slice(escaped, func(i, j int) bool {
		return len(escaped[i]) > len(escaped[j])
	})

	pattern := `\b(` + strings.Join(escaped, "|") + `)\b`
	var err error
	p.Regex, err = regexp.Compile("(?i)" + pattern)
	return err
}

// getFullInputText gathers all text from the LLMRequest contents to verify that generated replacements
// are not part of the input.
func getFullInputText(req *model.LLMRequest) string {
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

func (p *UniqueNamesPlugin) RedactUniqueNames(text string, fullInput string) string {
	if p.Regex == nil {
		return text
	}
	return p.Regex.ReplaceAllStringFunc(text, func(match string) string {
		return GetReplacement(p.Replacements, match, fullInput)
	})
}

// BeforeModelCallback intercepts the model request and redacts all found unique names.
func (p *UniqueNamesPlugin) BeforeModelCallback(ctx agent.Context, req *model.LLMRequest) (*model.LLMResponse, error) {
	if req == nil {
		return nil, nil
	}
	fullInput := getFullInputText(req)

	for _, content := range req.Contents {
		if content == nil {
			continue
		}
		for _, part := range content.Parts {
			if part == nil || part.Text == "" {
				continue
			}
			part.Text = p.RedactUniqueNames(part.Text, fullInput)
		}
	}
	return nil, nil
}

// UnredactText reverses the redact changes in the response text using the shared replacement table.
func (p *UniqueNamesPlugin) UnredactText(text string) string {
	type pair struct {
		rep  string
		orig string
	}

	var pairs []pair
	for rep, orig := range *p.Replacements {
		pairs = append(pairs, pair{rep, orig})
	}
	sort.Slice(pairs, func(i, j int) bool {
		return len(pairs[i].rep) > len(pairs[j].rep)
	})
	for _, pr := range pairs {
		text = strings.ReplaceAll(text, pr.rep, pr.orig)
	}

	return text
}

// AfterModelCallback restores the original names in the LLM response.
func (p *UniqueNamesPlugin) AfterModelCallback(ctx agent.Context, resp *model.LLMResponse, err error) (*model.LLMResponse, error) {
	if resp == nil || resp.Content == nil {
		return nil, nil
	}
	for _, part := range resp.Content.Parts {
		if part == nil || part.Text == "" {
			continue
		}
		part.Text = p.UnredactText(part.Text)
	}
	return nil, nil
}

// OnModelErrorCallback is a pass-through for model errors.
func (p *UniqueNamesPlugin) OnModelErrorCallback(ctx agent.Context, req *model.LLMRequest, err error) (*model.LLMResponse, error) {
	return nil, nil
}

// BeforeToolCallback restores the original names in the tool arguments. The
// model only ever sees the replacements, so the arguments it derives from them
// would not match anything on the machine the tool runs on. The arguments are
// updated in place and a nil result is returned, which keeps both the tool call
// itself and the callbacks of the remaining filters alive.
func (p *UniqueNamesPlugin) BeforeToolCallback(ctx agent.Context, t tool.Tool, args map[string]any) (map[string]any, error) {
	MapToolValues(args, p.UnredactText)
	return nil, nil
}

// AfterToolCallback redacts the tool result before it is handed to the model.
// A tool result travels as a FunctionResponse part, which carries its payload
// in a map and therefore never passes the text redaction of
// BeforeModelCallback.
//
// The arguments are redacted again as well. BeforeToolCallback restored them
// for the tool run, but they belong to the function call of the model, which
// stays in the session and is sent to the model again with every following
// request.
func (p *UniqueNamesPlugin) AfterToolCallback(ctx agent.Context, t tool.Tool, args, result map[string]any, err error) (map[string]any, error) {
	fullInput := ToolValuesText(args) + ToolValuesText(result)
	redact := func(text string) string {
		return p.RedactUniqueNames(text, fullInput)
	}
	MapToolValues(args, redact)
	if result == nil {
		return nil, nil
	}
	MapToolValues(result, redact)
	return nil, nil
}

// OnToolErrorCallback redacts the message of a failed tool call, which the flow
// would otherwise pass on to the model as {"error": ...}. Unlike the other tool
// callbacks this one has to return a result, which ends the callback chain of
// the runner, so filters that are meant to redact tool errors together have to
// be combined by piiplugin.NewPiiPlugin instead of being registered
// individually.
func (p *UniqueNamesPlugin) OnToolErrorCallback(ctx agent.Context, t tool.Tool, args map[string]any, err error) (map[string]any, error) {
	if err == nil {
		return nil, nil
	}
	message := err.Error()
	return map[string]any{"error": p.RedactUniqueNames(message, message)}, nil
}
