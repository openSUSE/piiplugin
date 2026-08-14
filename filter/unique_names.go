package filter

import (
	"regexp"
	"sort"
	"strings"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/tool"
)

type UniqueNamesFilter struct {
	Replacements *map[string]string
	Regex        *regexp.Regexp
}

func NewUniqueNamesFilter(replacements *map[string]string, names []string) (*UniqueNamesFilter, error) {
	f := &UniqueNamesFilter{Replacements: replacements}
	if f.Replacements == nil {
		m := make(map[string]string)
		f.Replacements = &m
	}
	if err := f.InitRegex(names); err != nil {
		return nil, err
	}
	return f, nil
}

func (f *UniqueNamesFilter) InitRegex(names []string) error {
	if len(names) == 0 {
		f.Regex = nil
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
	f.Regex, err = regexp.Compile("(?i)" + pattern)
	return err
}

func (f *UniqueNamesFilter) Redact(text string, fullInput string) string {
	if f.Regex == nil {
		return text
	}
	return f.Regex.ReplaceAllStringFunc(text, func(match string) string {
		return GetReplacement(f.Replacements, match, fullInput)
	})
}

func (f *UniqueNamesFilter) Unredact(text string) string {
	return UnredactText(f.Replacements, text)
}

// UniqueNamesPlugin manages a shared replacement table and performs
// redaction and unredaction on a list of unique names/strings.
type UniqueNamesPlugin struct {
	UniqueNamesFilter
}

// NewUniqueNamesPlugin creates a new UniqueNamesPlugin with the given options and name list.
// It initializes the regex field by calling InitRegex with the provided names.
// The Replacements map is either taken from the argument or created fresh if nil.
func NewUniqueNamesPlugin(replacements *map[string]string, names []string) (*UniqueNamesPlugin, error) {
	f, err := NewUniqueNamesFilter(replacements, names)
	if err != nil {
		return nil, err
	}
	return &UniqueNamesPlugin{UniqueNamesFilter: *f}, nil
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
	return p.Redact(text, fullInput)
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
	return p.Unredact(text)
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

// BeforeToolCallback restores the original names in the tool arguments.
func (p *UniqueNamesPlugin) BeforeToolCallback(ctx agent.Context, t tool.Tool, args map[string]any) (map[string]any, error) {
	MapToolValues(args, p.UnredactText)
	return nil, nil
}

// AfterToolCallback redacts the tool result before it is handed to the model.
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

// OnToolErrorCallback redacts the message of a failed tool call.
func (p *UniqueNamesPlugin) OnToolErrorCallback(ctx agent.Context, t tool.Tool, args map[string]any, err error) (map[string]any, error) {
	if err == nil {
		return nil, nil
	}
	message := err.Error()
	return map[string]any{"error": p.RedactUniqueNames(message, message)}, nil
}
