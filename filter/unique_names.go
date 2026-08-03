package filter

import (
	"regexp"
	"sort"
	"strings"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/model"
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
