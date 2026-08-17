// Package piiplugin provides the pure, ADK-independent PII filter composite and
// its configuration. It combines the username, eMail and host filters from the
// filter sub-packages over one shared replacement table. The ADK plugin/callback
// bindings live in the piiplugin/adk package so that consumers who only need the
// redaction engine never depend on the ADK.
package piiplugin

import (
	filteremail "github.com/openSUSE/piiplugin/filter/email"
	filterhost "github.com/openSUSE/piiplugin/filter/host"
	filterusername "github.com/openSUSE/piiplugin/filter/username"
)

// Configurer defines an interface for configuring the PII filters.
type Configurer interface {
	SetWithoutEmail(bool)
	SetWithoutUsername(bool)
	SetWithoutHost(bool)
	SetUsernameSource(filterusername.Source)
}

// PiiPluginOption is a functional option applied to a Configurer.
type PiiPluginOption func(Configurer)

// WithoutEmail disables the eMail filter.
func WithoutEmail() PiiPluginOption {
	return func(cfg Configurer) { cfg.SetWithoutEmail(true) }
}

// WithoutUsername disables the user name filter.
func WithoutUsername() PiiPluginOption {
	return func(cfg Configurer) { cfg.SetWithoutUsername(true) }
}

// WithoutHost disables the host filter.
func WithoutHost() PiiPluginOption {
	return func(cfg Configurer) { cfg.SetWithoutHost(true) }
}

// WithUsernameSource selects how the user name filter reads the user database
// (see filterusername.Source): filterusername.SourceAuto (the default) prefers
// the CGO getpwent(3) lookup and falls back to getent, filterusername.SourceCgo
// forces getpwent and filterusername.SourceGetent always reads the database
// through the getent command (no CGO required).
func WithUsernameSource(source filterusername.Source) PiiPluginOption {
	return func(cfg Configurer) { cfg.SetUsernameSource(source) }
}

// PiiFilter is the pure PII filter engine. It redacts and unredacts text using
// the enabled filter sub-filters over a single shared replacement table.
type PiiFilter struct {
	noEMail        bool
	noUserName     bool
	noHost         bool
	usernameSource filterusername.Source

	EmailFilter    *filteremail.EmailFilter
	HostFilter     *filterhost.HostFilter
	UsernameFilter *filterusername.UsernameFilter
	// Replacements is the shared replacement table.
	Replacements *map[string]string
}

func (f *PiiFilter) SetWithoutEmail(v bool)    { f.noEMail = v }
func (f *PiiFilter) SetWithoutUsername(v bool) { f.noUserName = v }
func (f *PiiFilter) SetWithoutHost(v bool)     { f.noHost = v }
func (f *PiiFilter) SetUsernameSource(s filterusername.Source) {
	if s != "" {
		f.usernameSource = s
	}
}

// NewPiiFilter creates a composite PiiFilter with the enabled filters sharing
// one replacement table.
func NewPiiFilter(opts ...PiiPluginOption) *PiiFilter {
	f := &PiiFilter{usernameSource: filterusername.SourceAuto}
	for _, o := range opts {
		o(f)
	}

	replacements := make(map[string]string)
	f.Replacements = &replacements

	if !f.noEMail {
		f.EmailFilter = filteremail.NewEmailFilter(
			filteremail.WithReplacement(&replacements),
		)
	}

	if !f.noUserName {
		f.UsernameFilter, _ = filterusername.NewUsernameFilter(
			filterusername.WithReplacement(&replacements),
			filterusername.WithUsernameSource(f.usernameSource),
		)
	}

	if !f.noHost {
		f.HostFilter, _ = filterhost.NewHostFilter(
			filterhost.WithReplacement(&replacements),
		)
	}

	return f
}

// Redact redacts text with the enabled filters in user name -> eMail -> host
// order. fullInput is the whole payload that generated replacements must not
// collide with; pass the individual text when redacting a single string.
func (f *PiiFilter) Redact(text, fullInput string) string {
	if f.UsernameFilter != nil {
		text = f.UsernameFilter.Redact(text, fullInput)
	}
	if f.EmailFilter != nil {
		text = f.EmailFilter.Redact(text, fullInput)
	}
	if f.HostFilter != nil {
		text = f.HostFilter.Redact(text, fullInput)
	}
	return text
}

// Unredact restores the original values in the reverse order: host -> eMail -> user name.
func (f *PiiFilter) Unredact(text string) string {
	if f.HostFilter != nil {
		text = f.HostFilter.Unredact(text)
	}
	if f.EmailFilter != nil {
		text = f.EmailFilter.Unredact(text)
	}
	if f.UsernameFilter != nil {
		text = f.UsernameFilter.Unredact(text)
	}
	return text
}
