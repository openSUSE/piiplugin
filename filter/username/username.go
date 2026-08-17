package filterusername

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/openSUSE/piiplugin/filter"
)

// UsernameFilter is the pure Go non-ADK filter engine. It builds its name list
// from the system's user database (login names and the GECOS field) and uses
// filter.UniqueNamesFilter to redact and unredact them.
type UsernameFilter struct {
	*filter.UniqueNamesFilter
	getpasswdFn    func() ([]string, error)
	isSystemUserFn func(id int) bool
	source         Source
}

// Option defines the functional option type for UsernameFilter.
type Option func(*UsernameFilter)

// UsernamePluginOption is a type alias for Option to maintain backward compatibility.
type UsernamePluginOption = Option

// Source selects how the filter reads the system's user database.
type Source string

const (
	// SourceAuto uses the CGO getpwent lookup when available and falls back to
	// the getent command otherwise.
	SourceAuto Source = "auto"
	// SourceCgo forces the CGO getpwent(3) lookup.
	SourceCgo Source = "cgo"
	// SourceGetent forces reading the user database through the getent command
	// (no CGO required).
	SourceGetent Source = "getent"
)

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

// WithGetpasswdFunc overrides the default user retrieval with a custom function
// that returns passwd/gecos compatible colon-separated entries. It wins over the
// source selected with WithUsernameSource.
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

// WithUsernameSource selects the user database source. SourceAuto (the default)
// prefers the CGO getpwent lookup and falls back to getent, SourceGetent always
// reads the database through getent so the filter can be built without CGO, and
// SourceCgo always uses getpwent(3).
func WithUsernameSource(source Source) Option {
	return func(u *UsernameFilter) {
		u.source = source
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

// FetchGetentPasswd returns passwd-compatible entries by parsing the output of
// `getent passwd`. It is a pure Go implementation of the user database lookup,
// so the filter can be built and used without CGO.
func FetchGetentPasswd() ([]string, error) {
	out, err := exec.Command("getent", "passwd").Output()
	if err != nil {
		return nil, fmt.Errorf("getent passwd: %w", err)
	}
	var entries []string
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if line != "" {
			entries = append(entries, line)
		}
	}
	return entries, nil
}

// resolvedGetpasswd picks the user database source for this filter: an explicit
// WithGetpasswdFunc always wins; otherwise SourceCgo uses getpwent(3),
// SourceGetent reads through getent, and SourceAuto (the default) uses getpwent(3)
// when it was compiled in (CGO enabled) and getent otherwise.
func (u *UsernameFilter) resolvedGetpasswd() (func() ([]string, error), error) {
	switch u.source {
	case SourceCgo:
		if cgoSource == nil {
			return nil, fmt.Errorf("username filter: CGO getpwent is not available in this build (built without CGO); use WithUsernameSource(SourceGetent) or provide a custom source")
		}
		return cgoSource, nil
	case SourceGetent:
		return FetchGetentPasswd, nil
	default: // SourceAuto and zero value
		if cgoSource != nil {
			return cgoSource, nil
		}
		return FetchGetentPasswd, nil
	}
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

	// Fetch from the configured source if no custom function is provided.
	if u.getpasswdFn == nil {
		getpasswd, err := u.resolvedGetpasswd()
		if err != nil {
			return nil, err
		}
		u.getpasswdFn = getpasswd
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
