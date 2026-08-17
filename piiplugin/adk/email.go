package adk

import (
	"google.golang.org/adk/v2/plugin"

	filteremail "github.com/openSUSE/piiplugin/filter/email"
)

// NewEmailPlugin wraps the pure eMail filter in an ADK plugin. The filter
// options are the same as in the filter package.
func NewEmailPlugin(opts ...filteremail.Option) (*plugin.Plugin, error) {
	return newRedactorPlugin("email_plugin", filteremail.NewEmailFilter(opts...))
}
