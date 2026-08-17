package adk

import (
	"google.golang.org/adk/v2/plugin"

	filterhost "github.com/openSUSE/piiplugin/filter/host"
)

// NewHostPlugin wraps the pure host filter in an ADK plugin. The filter options
// are the same as in the filter package.
func NewHostPlugin(opts ...filterhost.Option) (*plugin.Plugin, error) {
	f, err := filterhost.NewHostFilter(opts...)
	if err != nil {
		return nil, err
	}
	return newRedactorPlugin("host_plugin", f)
}
