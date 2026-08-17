// Package filteremail is the pure Go filter for eMail addresses. It has no
// dependency on the ADK; the plugin/callback layer lives in the piiplugin package.
package filteremail

import (
	"regexp"
	"strings"

	"github.com/openSUSE/piiplugin/filter"
)

// EmailFilter is the pureGo non-ADK filter engine.
type EmailFilter struct {
	replacements *map[string]string
	tldSuffix    map[string]string
}

// Option defines the functional option type for EmailFilter.
type Option func(*EmailFilter)

// EmailPluginOption is a type alias for Option to maintain backward compatibility.
type EmailPluginOption = Option

// WithTLDSuffix configures the tldSuffix replacement table for EmailFilter.
// It maps original TLDs (as keys) to their replacement values (as values).
// Use "*" as a key to set the fallback replacement for all unspecified TLDs.
func WithTLDSuffix(suffixMap map[string]string) Option {
	return func(f *EmailFilter) {
		for k, v := range suffixMap {
			f.tldSuffix[strings.ToLower(k)] = v
		}
	}
}

// WithReplacement sets a prefilled replacement table for EmailFilter.
// Passing a shared *map[string]string lets multiple filters use and extend the
// same replacement table, so that redaction and unredaction stay consistent
// across filters. The key is the generated replacement, the value the original.
func WithReplacement(replacements *map[string]string) Option {
	return func(f *EmailFilter) {
		if replacements != nil {
			f.replacements = replacements
		}
	}
}

// NewEmailFilter creates a new instance of the decoupled EmailFilter.
func NewEmailFilter(opts ...Option) *EmailFilter {
	f := &EmailFilter{
		tldSuffix: make(map[string]string),
	}
	for _, opt := range opts {
		opt(f)
	}
	if f.replacements == nil {
		m := make(map[string]string)
		f.replacements = &m
	}
	return f
}

// emailRegex matches email addresses and captures the local part and the domain part.
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

// splitDomain splits a domain into its labels and its TLD.
func splitDomain(domain string) (labels []string, tld string) {
	labels = strings.Split(domain, ".")
	if len(labels) > 1 && isAlpha(labels[len(labels)-1]) {
		tld = labels[len(labels)-1]
		labels = labels[:len(labels)-1]
	}
	return labels, tld
}

func (f *EmailFilter) anonymizeLabels(labels []string, fullInput string) []string {
	anonymized := make([]string, 0, len(labels))
	for _, label := range labels {
		if label == "" {
			anonymized = append(anonymized, "")
			continue
		}
		anonymized = append(anonymized, filter.GetReplacement(f.replacements, label, fullInput))
	}
	return anonymized
}

// Redact replaces all email parts with random pronounceable names according to requirements.
func (f *EmailFilter) Redact(text string, fullInput string) string {
	m := *f.replacements
	return emailRegex.ReplaceAllStringFunc(text, func(match string) string {
		submatches := emailRegex.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return match
		}
		localPart := submatches[1]
		domainPart := submatches[2]

		// Split localPart at the dots and replace each part
		anonymizedLocal := strings.Join(f.anonymizeLabels(strings.Split(localPart, "."), fullInput), ".")

		// Split the domain into its labels and its optional TLD and replace each label
		domainLabels, tld := splitDomain(domainPart)
		anonymizedDomainLabels := f.anonymizeLabels(domainLabels, fullInput)
		anonymizedDomain := strings.Join(anonymizedDomainLabels, ".")

		// Addresses without a TLD, e.g. "goo@baar" from a local log, keep their shape:
		// there is no TLD to map, all labels are already replaced.
		if tld == "" {
			return anonymizedLocal + "@" + anonymizedDomain
		}

		// Determine TLD suffix replacement strategy using the replacement table map
		replacementTLD, shouldReplaceTLD := f.tldSuffix[strings.ToLower(tld)]
		if !shouldReplaceTLD {
			replacementTLD, shouldReplaceTLD = f.tldSuffix["*"]
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

// Unredact reverses the redact changes in the response text using the shared replacement table.
func (f *EmailFilter) Unredact(text string) string {
	return filter.UnredactText(f.replacements, text)
}
