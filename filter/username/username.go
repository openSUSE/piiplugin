package filterusername

/*
#include <pwd.h>
#include <sys/types.h>
#include <stdlib.h>
*/
import "C"

import (
	"fmt"
	"strings"

	"github.com/openSUSE/piiplug/filter"
	"google.golang.org/adk/v2/plugin"
)

type UsernamePlugin struct {
	*filter.UniqueNamesPlugin
	getpasswdFn  func() ([]string, error)
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
			p.Replacements = replacements
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

// FetchCgoPasswd returns passwd entries from the system via CGO getpwent.
func FetchCgoPasswd() ([]string, error) {
	var entries []string
	C.setpwent()
	defer C.endpwent()

	for {
		pw := C.getpwent()
		if pw == nil {
			break
		}
		// Format: name:passwd:uid:gid:gecos:dir:shell
		entry := fmt.Sprintf(
			"%s:x:%d:%d:%s:%s:%s",
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

// NewUsernamePlugin creates a new instance of the username filter plugin.
func NewUsernamePlugin(opts ...UsernamePluginOption) (*plugin.Plugin, error) {
	p := &UsernamePlugin{
		UniqueNamesPlugin: &filter.UniqueNamesPlugin{},
	}
	for _, opt := range opts {
		opt(p)
	}

	if p.Replacements == nil {
		m := make(map[string]string)
		p.Replacements = &m
	}

	// Fetch from CGO if no custom function is provided
	if p.getpasswdFn == nil {
		p.getpasswdFn = FetchCgoPasswd
	}

	entries, err := p.getpasswdFn()
	if err != nil {
		return nil, err
	}

	names := parsePasswdEntries(entries)

	err = p.UniqueNamesPlugin.InitRegex(names)
	if err != nil {
		return nil, err
	}

	return plugin.New(plugin.Config{
		Name:                 "username_plugin",
		BeforeModelCallback:  p.BeforeModelCallback,
		AfterModelCallback:   p.AfterModelCallback,
		OnModelErrorCallback: p.OnModelErrorCallback,
		BeforeToolCallback:   p.BeforeToolCallback,
		AfterToolCallback:    p.AfterToolCallback,
		OnToolErrorCallback:  p.OnToolErrorCallback,
	})
}
