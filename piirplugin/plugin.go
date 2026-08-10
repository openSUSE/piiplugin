package piirplugin

import (
	filteremail "github.com/openSUSE/piirplug/filter/email"
	filterhost "github.com/openSUSE/piirplug/filter/host"
	filterusername "github.com/openSUSE/piirplug/filter/username"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/plugin"
	"google.golang.org/adk/v2/tool"
)

type PiirPluginOption func(*PiirPlugin)

type PiirPlugin struct {
	noEMail        bool
	noUserName     bool
	noHost         bool
	eMailPlugin    *plugin.Plugin
	userNamePlugin *plugin.Plugin
	hostPlugin     *plugin.Plugin
}

func WithoutEmail() PiirPluginOption {
	return func(cfg *PiirPlugin) {
		cfg.noEMail = true
	}
}

func WithoutUsername() PiirPluginOption {
	return func(cfg *PiirPlugin) {
		cfg.noUserName = true
	}
}

func WithoutHost() PiirPluginOption {
	return func(cfg *PiirPlugin) {
		cfg.noHost = true
	}
}

func NewPiirPlugin(opts ...PiirPluginOption) *plugin.Plugin {
	p := &PiirPlugin{}
	for _, o := range opts {
		o(p)
	}

	replacements := make(map[string]string)

	if !p.noEMail {
		p.eMailPlugin, _ = filteremail.NewEmailPlugin(
			filteremail.WithReplacement(&replacements),
		)
	}

	if !p.noUserName {
		p.userNamePlugin, _ = filterusername.NewUsernamePlugin(
			filterusername.WithReplacement(&replacements),
		)
	}

	if !p.noHost {
		p.hostPlugin, _ = filterhost.NewHostPlugin(
			filterhost.WithReplacement(&replacements),
		)
	}

	plug, _ := plugin.New(plugin.Config{
		Name:                 "pii_plugin",
		BeforeModelCallback:  p.BeforeModelCallback,
		AfterModelCallback:   p.AfterModelCallback,
		OnModelErrorCallback: p.OnModelErrorCallback,
		BeforeToolCallback:   p.BeforeToolCallback,
		AfterToolCallback:    p.AfterToolCallback,
		OnToolErrorCallback:  p.OnToolErrorCallback,
	})
	return plug
}

// redactOrder lists the filters in the order in which data leaving the machine
// is redacted, unredactOrder the reverse order used for incoming data.
func (p *PiirPlugin) redactOrder() []*plugin.Plugin {
	return []*plugin.Plugin{p.userNamePlugin, p.eMailPlugin, p.hostPlugin}
}

func (p *PiirPlugin) unredactOrder() []*plugin.Plugin {
	return []*plugin.Plugin{p.hostPlugin, p.eMailPlugin, p.userNamePlugin}
}

func (p *PiirPlugin) BeforeModelCallback(ctx agent.Context, req *model.LLMRequest) (*model.LLMResponse, error) {
	for _, plg := range p.redactOrder() {
		if plg == nil {
			continue
		}
		if cb := plg.BeforeModelCallback(); cb != nil {
			if resp, err := cb(ctx, req); err != nil || resp != nil {
				return resp, err
			}
		}
	}
	return nil, nil
}

func (p *PiirPlugin) AfterModelCallback(ctx agent.Context, resp *model.LLMResponse, err error) (*model.LLMResponse, error) {
	for _, plg := range p.unredactOrder() {
		if plg == nil {
			continue
		}
		if cb := plg.AfterModelCallback(); cb != nil {
			if r, e := cb(ctx, resp, err); e != nil {
				return nil, e
			} else if r != nil {
				resp = r
			}
		}
	}
	return resp, nil
}

// BeforeToolCallback restores the original values in the tool arguments so that
// the tool runs against the real system instead of against the replacements the
// model has seen. The filters update the arguments in place, so a nil result is
// returned and the tool call itself still happens.
func (p *PiirPlugin) BeforeToolCallback(ctx agent.Context, t tool.Tool, args map[string]any) (map[string]any, error) {
	for _, plg := range p.unredactOrder() {
		if plg == nil {
			continue
		}
		if cb := plg.BeforeToolCallback(); cb != nil {
			if res, err := cb(ctx, t, args); err != nil || res != nil {
				return res, err
			}
		}
	}
	return nil, nil
}

// AfterToolCallback redacts the tool result, and the arguments restored by
// BeforeToolCallback, before they are sent to the model. Neither of them is
// part of the model request contents as text, so the filters have to be applied
// here as well.
func (p *PiirPlugin) AfterToolCallback(ctx agent.Context, t tool.Tool, args, result map[string]any, err error) (map[string]any, error) {
	redacted, replaced := result, false
	for _, plg := range p.redactOrder() {
		if plg == nil {
			continue
		}
		if cb := plg.AfterToolCallback(); cb != nil {
			r, e := cb(ctx, t, args, redacted, err)
			if e != nil {
				return nil, e
			}
			if r != nil {
				redacted, replaced = r, true
			}
		}
	}
	if replaced {
		return redacted, nil
	}
	return nil, nil
}

// OnToolErrorCallback redacts the message of a failed tool call. The flow turns
// a tool error into an {"error": ...} result for the model anyway, so that
// result is built here and redacted by every filter.
func (p *PiirPlugin) OnToolErrorCallback(ctx agent.Context, t tool.Tool, args map[string]any, err error) (map[string]any, error) {
	if err == nil {
		return nil, nil
	}
	result := map[string]any{"error": err.Error()}
	if r, e := p.AfterToolCallback(ctx, t, args, result, nil); e != nil {
		return nil, e
	} else if r != nil {
		result = r
	}
	return result, nil
}

func (p *PiirPlugin) OnModelErrorCallback(ctx agent.Context, req *model.LLMRequest, err error) (*model.LLMResponse, error) {
	var resp *model.LLMResponse
	for _, plg := range p.redactOrder() {
		if plg == nil {
			continue
		}
		if cb := plg.OnModelErrorCallback(); cb != nil {
			if r, e := cb(ctx, req, err); e != nil {
				return nil, e
			} else if r != nil {
				resp = r
			}
		}
	}
	return resp, nil
}
