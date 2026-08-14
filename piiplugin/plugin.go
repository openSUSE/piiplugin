package piiplugin

import (
	filteremail "github.com/openSUSE/piiplugin/filter/email"
	filterhost "github.com/openSUSE/piiplugin/filter/host"
	filterusername "github.com/openSUSE/piiplugin/filter/username"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/plugin"
	"google.golang.org/adk/v2/tool"
)

// Configurer defines an interface for configuring PII filters.
type Configurer interface {
	SetWithoutEmail(bool)
	SetWithoutUsername(bool)
	SetWithoutHost(bool)
}

type PiiPluginOption func(Configurer)

type PiiPlugin struct {
	noEMail        bool
	noUserName     bool
	noHost         bool
	eMailPlugin    *plugin.Plugin
	userNamePlugin *plugin.Plugin
	hostPlugin     *plugin.Plugin
}

func (p *PiiPlugin) SetWithoutEmail(v bool) {
	p.noEMail = v
}

func (p *PiiPlugin) SetWithoutUsername(v bool) {
	p.noUserName = v
}

func (p *PiiPlugin) SetWithoutHost(v bool) {
	p.noHost = v
}

func WithoutEmail() PiiPluginOption {
	return func(cfg Configurer) {
		cfg.SetWithoutEmail(true)
	}
}

func WithoutUsername() PiiPluginOption {
	return func(cfg Configurer) {
		cfg.SetWithoutUsername(true)
	}
}

func WithoutHost() PiiPluginOption {
	return func(cfg Configurer) {
		cfg.SetWithoutHost(true)
	}
}

type PiiFilter struct {
	noEMail        bool
	noUserName     bool
	noHost         bool
	EmailFilter    *filteremail.EmailFilter
	HostFilter     *filterhost.HostFilter
	UsernameFilter *filterusername.UsernameFilter
}

func (f *PiiFilter) SetWithoutEmail(v bool) {
	f.noEMail = v
}

func (f *PiiFilter) SetWithoutUsername(v bool) {
	f.noUserName = v
}

func (f *PiiFilter) SetWithoutHost(v bool) {
	f.noHost = v
}

func NewPiiFilter(opts ...PiiPluginOption) *PiiFilter {
	f := &PiiFilter{}
	for _, o := range opts {
		o(f)
	}

	replacements := make(map[string]string)

	if !f.noEMail {
		f.EmailFilter = filteremail.NewEmailFilter(
			filteremail.WithReplacement(&replacements),
		)
	}

	if !f.noUserName {
		f.UsernameFilter, _ = filterusername.NewUsernameFilter(
			filterusername.WithReplacement(&replacements),
		)
	}

	if !f.noHost {
		f.HostFilter, _ = filterhost.NewHostFilter(
			filterhost.WithReplacement(&replacements),
		)
	}

	return f
}

func (f *PiiFilter) Redact(text string) string {
	// Username -> Email -> Host
	if f.UsernameFilter != nil {
		text = f.UsernameFilter.Redact(text, text)
	}
	if f.EmailFilter != nil {
		text = f.EmailFilter.Redact(text, text)
	}
	if f.HostFilter != nil {
		text = f.HostFilter.Redact(text, text)
	}
	return text
}

func (f *PiiFilter) Unredact(text string) string {
	// Host -> Email -> Username
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

func NewPiiPlugin(opts ...PiiPluginOption) *plugin.Plugin {
	p := &PiiPlugin{}
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
func (p *PiiPlugin) redactOrder() []*plugin.Plugin {
	return []*plugin.Plugin{p.userNamePlugin, p.eMailPlugin, p.hostPlugin}
}

func (p *PiiPlugin) unredactOrder() []*plugin.Plugin {
	return []*plugin.Plugin{p.hostPlugin, p.eMailPlugin, p.userNamePlugin}
}

func (p *PiiPlugin) BeforeModelCallback(ctx agent.Context, req *model.LLMRequest) (*model.LLMResponse, error) {
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

func (p *PiiPlugin) AfterModelCallback(ctx agent.Context, resp *model.LLMResponse, err error) (*model.LLMResponse, error) {
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
func (p *PiiPlugin) BeforeToolCallback(ctx agent.Context, t tool.Tool, args map[string]any) (map[string]any, error) {
	for _, plg := range p.unredactOrder() {
		if plg == nil {
			continue
		}
		if cb := plg.BeforeToolCallback(); cb != nil {
			if res, err := cb(ctx, t, args); err != nil {
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
func (p *PiiPlugin) AfterToolCallback(ctx agent.Context, t tool.Tool, args, result map[string]any, err error) (map[string]any, error) {
	for _, plg := range p.redactOrder() {
		if plg == nil {
			continue
		}
		if cb := plg.AfterToolCallback(); cb != nil {
			r, e := cb(ctx, t, args, result, err)
			if e != nil {
				return nil, e
			}
			if r != nil {
				return r, nil
			}
		}
	}
	return nil, nil
}

// OnToolErrorCallback redacts the message of a failed tool call. The flow turns
// a tool error into an {"error": ...} result for the model anyway, so that
// result is built here and redacted by every filter.
//
// Note: The callback returns (map[string]any, nil) not (nil, error) because
// ADK's OnToolErrorCallback must return a map for the LLM response content;
// tool errors are communicated to the model via result maps containing an
// "error" key, not via Go errors.
func (p *PiiPlugin) OnToolErrorCallback(ctx agent.Context, t tool.Tool, args map[string]any, err error) (map[string]any, error) {
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

// OnModelErrorCallback is a pass-through for model errors. It delegates to the
// underlying filters in redact order; the individual filters do not modify error
// handling, so this callback effectively returns nil.
func (p *PiiPlugin) OnModelErrorCallback(ctx agent.Context, req *model.LLMRequest, err error) (*model.LLMResponse, error) {
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
