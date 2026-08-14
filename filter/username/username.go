package filterusername

/*
#include <pwd.h>
#include <sys/types.h>
#include <stdlib.h>
*/
import "C"

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/openSUSE/piiplugin/filter"
	"google.golang.org/adk/v2/plugin"
)

// UsernameFilter is the pure Go non-ADK filter engine.
type UsernameFilter struct {
	*filter.UniqueNamesFilter
	getpasswdFn    func() ([]string, error)
	isSystemUserFn func(id int) bool
}

// Option defines the functional option type for UsernameFilter.
type Option func(*UsernameFilter)

// UsernamePluginOption is a type alias for Option to maintain backward compatibility.
type UsernamePluginOption = Option

// WithReplacement sets a prefilled replacement table for UsernameFilter.
func WithReplacement(replacements *map[string]string) Option {
	return func(u *UsernameFilter) {
		if replacements != nil {
			if u.UniqueNamesFilter == nil {
				u.UniqueNamesFilter = &filter.UniqueNamesFilter{}
			}
			u.Replacements = replacements
		}
	}
}

// WithGetpasswdFunc allows replacing the default user retrieval (CGO getpwent) with a
// custom function that returns passwd/gecos compatible colon-separated entries.
func WithGetpasswdFunc(fn func() ([]string, error)) Option {
	return func(u *UsernameFilter) {
		u.getpasswdFn = fn
	}
}

// WithIsSystemUserFunc allows replacing the default function that identifies system users.
func WithIsSystemUserFunc(fn func(id int) bool) Option {
	return func(u *UsernameFilter) {
		u.isSystemUserFn = fn
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
func parsePasswdEntries(entries []string, isSystemUser func(id int) bool) []string {
	var names []string
	for _, entry := range entries {
		parts := strings.Split(entry, ":")
		if len(parts) < 5 {
			continue
		}
		username := parts[0]
		uidStr := parts[2]
		gecos := parts[4]

		uidInt, err := strconv.Atoi(uidStr)
		if err != nil {
			continue
		}

		// Don't replace system users (regular users have UID >= 1000)
		if isSystemUser(uidInt) {
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

// IsSystemUserDefault is the default function to identify system users.
func IsSystemUserDefault(id int) bool {
	return id < 1000
}

// NewUsernameFilter creates a new instance of the decoupled UsernameFilter.
func NewUsernameFilter(opts ...Option) (*UsernameFilter, error) {
	u := &UsernameFilter{}
	for _, opt := range opts {
		opt(u)
	}

	if u.UniqueNamesFilter == nil {
		m := make(map[string]string)
		f, err := filter.NewUniqueNamesFilter(&m, nil)
		if err != nil {
			return nil, err
		}
		u.UniqueNamesFilter = f
	}

	// Fetch from CGO if no custom function is provided
	if u.getpasswdFn == nil {
		u.getpasswdFn = FetchCgoPasswd
	}

	entries, err := u.getpasswdFn()
	if err != nil {
		return nil, err
	}

	if u.isSystemUserFn == nil {
		u.isSystemUserFn = IsSystemUserDefault
	}

	names := parsePasswdEntries(entries, u.isSystemUserFn)

	err = u.UniqueNamesFilter.InitRegex(names)
	if err != nil {
		return nil, err
	}

	return u, nil
}

// NewUsernamePlugin creates a new instance of the username filter plugin.
func NewUsernamePlugin(opts ...Option) (*plugin.Plugin, error) {
	f, err := NewUsernameFilter(opts...)
	if err != nil {
		return nil, err
	}
	p := &filter.UniqueNamesPlugin{
		UniqueNamesFilter: *f.UniqueNamesFilter,
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
