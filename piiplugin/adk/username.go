package adk

import (
	"google.golang.org/adk/v2/plugin"

	filterusername "github.com/openSUSE/piiplugin/filter/username"
)

// NewUsernamePlugin wraps the pure user name filter in an ADK plugin. The
// filter options are the same as in the filter package, including
// filterusername.WithUsernameSource to select the CGO or getent source of the
// user database.
func NewUsernamePlugin(opts ...filterusername.Option) (*plugin.Plugin, error) {
	f, err := filterusername.NewUsernameFilter(opts...)
	if err != nil {
		return nil, err
	}
	return newRedactorPlugin("username_plugin", f)
}
