package filterusername

/*
#include <pwd.h>
#include <sys/types.h>
#include <stdlib.h>
*/
import "C"

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/openSUSE/piiplug/filter"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/plugin"
)

type UsernamePlugin struct {
	// replacements maps the generated replacement (key) to the original string
	// (value). It is a pointer to a single map so that the same table can be
	// shared with other filters, keeping redaction consistent across them.
	replacements *map[string]string
	getpasswdFn  func() ([]string, error)
	regex        *regexp.Regexp
}

// UsernamePluginOption defines the functional option type for UsernamePlugin.
type UsernamePluginOption func(*UsernamePlugin)

// WithReplacement sets a prefilled replacement table for UsernamePlugin.
// Passing a shared *map[string]string lets multiple filters use and extend the
// same replacement table, so that redaction and unredaction stay consistent
// across filters. The key is the generated replacement, the value the original.
func WithReplacement(replacements *map[string]string) UsernamePluginOption {
	return func(p *UsernamePlugin) {
		if replacements != nil {
			p.replacements = replacements
		}
	}
}

// WithGetpasswdFunc allows replacing the default user retrieval (CGO getpwent) with a
// custom function that returns passwd/gecos compatible colon-separated entries.
func WithGetpasswdFunc(fn func() ([]string, error)) UsernamePluginOption {
	return func(p *UsernamePlugin) {
		p.getpasswdFn = fn
	}
}

func uniqueNames(names []string) []string {
	seen := make(map[string]bool)
	var unique []string
	for _, name := range names {
		lower := strings.ToLower(name)
		if !seen[lower] {
			seen[lower] = true
			unique = append(unique, name)
		}
	}
	return unique
}

// fetchCgoPasswd returns passwd entries from the system via CGO getpwent.
func fetchCgoPasswd() ([]string, error) {
	var entries []string
	C.setpwent()
	defer C.endpwent()

	for {
		pw := C.getpwent()
		if pw == nil {
			break
		}
		// Format: name:passwd:uid:gid:gecos:dir:shell
		entry := fmt.Sprintf("%s:x:%d:%d:%s:%s:%s",
			C.GoString(pw.pw_name),
			uint32(pw.pw_uid),
			uint32(pw.pw_gid),
			C.GoString(pw.pw_gecos),
			C.GoString(pw.pw_dir),
			C.GoString(pw.pw_shell),
		)
		entries = append(entries, entry)
	}
	return entries, nil
}

// parsePasswdEntries extracts usernames and individual GECOS names from passwd-compatible lines.
func parsePasswdEntries(entries []string) []string {
	var names []string
	for _, entry := range entries {
		parts := strings.Split(entry, ":")
		if len(parts) < 5 {
			continue
		}
		username := parts[0]
		uidStr := parts[2]
		gecos := parts[4]

		var uid uint32
		if _, err := fmt.Sscanf(uidStr, "%d", &uid); err != nil {
			continue
		}

		// Don't replace system users (regular users have UID >= 1000)
		if uid < 1000 {
			continue
		}

		if len(username) >= 2 {
			names = append(names, username)
		}

		// Split the GECOS field by commas first, to get the full name sub-field
		gecosName := gecos
		if commaIdx := strings.Index(gecos, ","); commaIdx != -1 {
			gecosName = gecos[:commaIdx]
		}

		// Split the full name sub-field at spaces to get names
		fields := strings.Fields(gecosName)
		for _, f := range fields {
			// Clean name fields of non-alphanumeric punctuation
			f = strings.Trim(f, ".,;:!?()")
			if len(f) >= 2 {
				names = append(names, f)
			}
		}
	}
	return uniqueNames(names)
}

func buildRegex(names []string) (*regexp.Regexp, error) {
	if len(names) == 0 {
		return nil, nil
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
	return regexp.Compile("(?i)" + pattern)
}

// NewUsernamePlugin creates a new instance of the username filter plugin.
func NewUsernamePlugin(opts ...UsernamePluginOption) (*plugin.Plugin, error) {
	p := &UsernamePlugin{}
	for _, opt := range opts {
		opt(p)
	}

	if p.replacements == nil {
		m := make(map[string]string)
		p.replacements = &m
	}

	// Fetch from CGO if no custom function is provided
	if p.getpasswdFn == nil {
		p.getpasswdFn = fetchCgoPasswd
	}

	entries, err := p.getpasswdFn()
	if err != nil {
		return nil, err
	}

	names := parsePasswdEntries(entries)

	p.regex, err = buildRegex(names)
	if err != nil {
		return nil, err
	}

	return plugin.New(plugin.Config{
		Name:                 "username_plugin",
		BeforeModelCallback:  p.BeforeModelCallback,
		AfterModelCallback:   p.AfterModelCallback,
		OnModelErrorCallback: p.OnModelErrorCallback,
	})
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

func (p *UsernamePlugin) redactUsernames(text string, fullInput string) string {
	if p.regex == nil {
		return text
	}
	return p.regex.ReplaceAllStringFunc(text, func(match string) string {
		return filter.GetReplacement(p.replacements, match, fullInput)
	})
}

// BeforeModelCallback intercepts the model request and redacts all found usernames.
func (p *UsernamePlugin) BeforeModelCallback(ctx agent.Context, req *model.LLMRequest) (*model.LLMResponse, error) {
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
			part.Text = p.redactUsernames(part.Text, fullInput)
		}
	}
	return nil, nil
}

// unredactText reverses the redact changes in the response text using the shared replacement table.
func (p *UsernamePlugin) unredactText(text string) string {
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

// AfterModelCallback restores the original usernames in the LLM response.
func (p *UsernamePlugin) AfterModelCallback(ctx agent.Context, resp *model.LLMResponse, err error) (*model.LLMResponse, error) {
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
func (p *UsernamePlugin) OnModelErrorCallback(ctx agent.Context, req *model.LLMRequest, err error) (*model.LLMResponse, error) {
	return nil, nil
}
