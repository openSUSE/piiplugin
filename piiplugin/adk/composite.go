// Package adk — composite plugin.
package adk

import (
	"google.golang.org/adk/v2/plugin"

	"github.com/openSUSE/piiplugin/piiplugin"
)

// piiFilterRedactor adapts the pure piiplugin.PiiFilter to the Redactor
// interface so it can be wrapped by the generic redactorPlugin. The filter
// ordering (user name -> eMail -> host for redaction, the reverse for
// unredaction) is handled inside PiiFilter.
type piiFilterRedactor struct {
	f *piiplugin.PiiFilter
}

func (r *piiFilterRedactor) Redact(text, fullInput string) string {
	return r.f.Redact(text, fullInput)
}

func (r *piiFilterRedactor) Unredact(text string) string {
	return r.f.Unredact(text)
}

// NewPiiPlugin builds a composite ADK plugin from the enabled filters, all
// sharing one replacement table. The options are the same as the pure
// piiplugin package, e.g. piiplugin.WithoutEmail, piiplugin.WithoutUsername,
// piiplugin.WithoutHost, or piiplugin.WithUsernameSource to select the user
// database source.
func NewPiiPlugin(opts ...piiplugin.PiiPluginOption) (*plugin.Plugin, error) {
	return newRedactorPlugin("pii_plugin", &piiFilterRedactor{f: piiplugin.NewPiiFilter(opts...)})
}
