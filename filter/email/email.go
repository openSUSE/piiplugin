package filteremail

import (
	"regexp"
	"sort"
	"strings"

	"github.com/openSUSE/piiplug/filter"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/plugin"
)

type EmailPlugin struct {
	// replacements maps the generated replacement (key) to the original string
	// (value). It is a pointer to a single map so that the same table can be
	// shared with other filters, keeping redaction consistent across them.
	replacements *map[string]string
	tldSuffix    map[string]string
}

// EmailPluginOption defines the functional option type for EmailPlugin
type EmailPluginOption func(*EmailPlugin)

// WithTLDSuffix configures the tldSuffix replacement table for EmailPlugin.
// It maps original TLDs (as keys) to their replacement values (as values).
// Use "*" as a key to set the fallback replacement for all unspecified TLDs.
func WithTLDSuffix(suffixMap map[string]string) EmailPluginOption {
	return func(p *EmailPlugin) {
		for k, v := range suffixMap {
			p.tldSuffix[strings.ToLower(k)] = v
		}
	}
}

// WithReplacement sets a prefilled replacement table for EmailPlugin.
// Passing a shared *map[string]string lets multiple filters use and extend the
// same replacement table, so that redaction and unredaction stay consistent
// across filters. The key is the generated replacement, the value the original.
func WithReplacement(replacements *map[string]string) EmailPluginOption {
	return func(p *EmailPlugin) {
		if replacements != nil {
			p.replacements = replacements
		}
	}
}

// NewEmailPlugin creates a new instance of the email filter plugin.
// It defaults the fallback tldSuffix "*" to "tld" if not specified, ensuring "*" is always set.
func NewEmailPlugin(opts ...EmailPluginOption) (*plugin.Plugin, error) {
	p := &EmailPlugin{
		tldSuffix: map[string]string{"*": "tld"}, // Default value
	}
	for _, opt := range opts {
		opt(p)
	}
	if p.replacements == nil {
		m := make(map[string]string)
		p.replacements = &m
	}
	if _, exists := p.tldSuffix["*"]; !exists {
		p.tldSuffix["*"] = "tld"
	}
	return plugin.New(plugin.Config{
		Name:                 "eMail_plugin",
		BeforeModelCallback:  p.BeforeModelCallback,
		AfterModelCallback:   p.AfterModelCallback,
		OnModelErrorCallback: p.OnModelErrorCallback,
	})
}

// emailRegex matches email addresses and captures the local part and the domain part.
// The TLD is deliberately not part of the pattern so that unusual addresses without
// a TLD, as they occur in local logs (e.g. "goo@baar"), are recognized as well.
// The domain is split into its labels and its optional TLD afterwards, see splitDomain.
var emailRegex = regexp.MustCompile(`\b([a-zA-Z0-9._%+-]+)@([a-zA-Z0-9-]+(?:\.[a-zA-Z0-9-]+)*)`)

// isAlpha reports whether s is non-empty and consists only of ASCII letters.
func isAlpha(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
			return false
		}
	}
	return true
}

// splitDomain splits a domain into its labels and its TLD. A TLD is only reported
// if the domain has more than one label and the last one is purely alphabetic.
// Local addresses like "baar" or hosts like "192.168.0.1" therefore have no TLD and
// an empty tld is returned, in which case all labels are treated as regular labels.
func splitDomain(domain string) (labels []string, tld string) {
	labels = strings.Split(domain, ".")
	if len(labels) > 1 && isAlpha(labels[len(labels)-1]) {
		tld = labels[len(labels)-1]
		labels = labels[:len(labels)-1]
	}
	return labels, tld
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

// anonymizeLabels replaces every non-empty label with a generated replacement and
// records the mapping in the shared replacement table. Empty labels, which stem from
// consecutive dots, are kept as they are.
func (p *EmailPlugin) anonymizeLabels(labels []string, fullInput string) []string {
	anonymized := make([]string, 0, len(labels))
	for _, label := range labels {
		if label == "" {
			anonymized = append(anonymized, "")
			continue
		}
		anonymized = append(anonymized, filter.GetReplacement(p.replacements, label, fullInput))
	}
	return anonymized
}

// redactEmails replaces all email parts with random pronounceable names according to requirements.
func (p *EmailPlugin) redactEmails(text string, fullInput string) string {
	m := *p.replacements
	return emailRegex.ReplaceAllStringFunc(text, func(match string) string {
		submatches := emailRegex.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return match
		}
		localPart := submatches[1]
		domainPart := submatches[2]

		// Split localPart at the dots and replace each part
		anonymizedLocal := strings.Join(p.anonymizeLabels(strings.Split(localPart, "."), fullInput), ".")

		// Split the domain into its labels and its optional TLD and replace each label
		domainLabels, tld := splitDomain(domainPart)
		anonymizedDomainLabels := p.anonymizeLabels(domainLabels, fullInput)
		anonymizedDomain := strings.Join(anonymizedDomainLabels, ".")

		// Addresses without a TLD, e.g. "goo@baar" from a local log, keep their shape:
		// there is no TLD to map, all labels are already replaced.
		if tld == "" {
			return anonymizedLocal + "@" + anonymizedDomain
		}

		// Determine TLD suffix replacement strategy using the replacement table map
		replacementTLD, shouldReplaceTLD := p.tldSuffix[strings.ToLower(tld)]
		if !shouldReplaceTLD {
			replacementTLD, shouldReplaceTLD = p.tldSuffix["*"]
		}
		if !shouldReplaceTLD {
			// Keep the original TLD
			return anonymizedLocal + "@" + anonymizedDomain + "." + tld
		}

		// Replace the TLD with the mapped replacement suffix. splitDomain only reports a
		// TLD for domains with at least two labels, so the last label always exists here.
		// Map it combined with the replacement suffix so that unredacting restores the
		// original label and TLD in one go.
		lastIdx := len(anonymizedDomainLabels) - 1
		m[anonymizedDomainLabels[lastIdx]+"."+replacementTLD] = domainLabels[lastIdx] + "." + tld

		return anonymizedLocal + "@" + anonymizedDomain + "." + replacementTLD
	})
}

// BeforeModelCallback intercepts the model request and redacts all found email addresses.
func (p *EmailPlugin) BeforeModelCallback(ctx agent.Context, req *model.LLMRequest) (*model.LLMResponse, error) {
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
			part.Text = p.redactEmails(part.Text, fullInput)
		}
	}
	return nil, nil
}

// unredactText reverses the redact changes in the response text using the shared
// replacement table. It sorts the mapped keys by length descending to prevent
// partial replacements (e.g. combined "segment.suffix" keys before bare segments).
func (p *EmailPlugin) unredactText(text string) string {
	type pair struct {
		rep  string
		orig string
	}

	var pairs []pair
	for rep, orig := range *p.replacements {
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

// AfterModelCallback restores the original emails in the LLM response.
func (p *EmailPlugin) AfterModelCallback(ctx agent.Context, resp *model.LLMResponse, err error) (*model.LLMResponse, error) {
	if resp == nil || resp.Content == nil {
		return nil, nil
	}
	for _, part := range resp.Content.Parts {
		if part == nil || part.Text == "" {
			continue
		}
		part.Text = p.unredactText(part.Text)
	}
	return nil, nil
}

// OnModelErrorCallback is a pass-through for model errors.
func (p *EmailPlugin) OnModelErrorCallback(ctx agent.Context, req *model.LLMRequest, err error) (*model.LLMResponse, error) {
	return nil, nil
}
