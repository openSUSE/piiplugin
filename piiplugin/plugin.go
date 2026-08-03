package piiplugin

import (
	filteremail "github.com/openSUSE/piiplug/filter/email"
	filterhost "github.com/openSUSE/piiplug/filter/host"
	filterusername "github.com/openSUSE/piiplug/filter/username"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/plugin"
)

type PiiPluginOption func(*PiiPlugin)

type PiiPlugin struct {
	noEMail        bool
	noUserName     bool
	noHost         bool
	eMailPlugin    *plugin.Plugin
	userNamePlugin *plugin.Plugin
	hostPlugin     *plugin.Plugin
}

func WithoutEmail() PiiPluginOption {
	return func(cfg *PiiPlugin) {
		cfg.noEMail = true
	}
}

func WithoutUsername() PiiPluginOption {
	return func(cfg *PiiPlugin) {
		cfg.noUserName = true
	}
}

func WithoutHost() PiiPluginOption {
	return func(cfg *PiiPlugin) {
		cfg.noHost = true
	}
}

func NewPiiPlugin(opts ...PiiPluginOption) *plugin.Plugin {
	p := &PiiPlugin{}
	for _, o := range opts {
		o(p)
	}

	replacements := make(map[string]string)

	if !p.noEMail {
		var err error
		p.eMailPlugin, err = filteremail.NewEmailPlugin(
			filteremail.WithReplacement(&replacements),
		)
		if err != nil {
			// ignore error as NewEmailPlugin default options won't fail
		}
	}

	if !p.noUserName {
		var err error
		p.userNamePlugin, err = filterusername.NewUsernamePlugin(
			filterusername.WithReplacement(&replacements),
		)
		if err != nil {
			// ignore error as NewUsernamePlugin default options won't fail
		}
	}

	if !p.noHost {
		var err error
		p.hostPlugin, err = filterhost.NewHostPlugin(
			filterhost.WithReplacement(&replacements),
		)
		if err != nil {
			// ignore error as NewHostPlugin default options won't fail
		}
	}

	plug, _ := plugin.New(plugin.Config{
		Name:                 "pii_plugin",
		BeforeModelCallback:  p.BeforeModelCallback,
		AfterModelCallback:   p.AfterModelCallback,
		OnModelErrorCallback: p.OnModelErrorCallback,
	})
	return plug
}

func (p *PiiPlugin) BeforeModelCallback(ctx agent.Context, req *model.LLMRequest) (*model.LLMResponse, error) {
	if p.eMailPlugin != nil {
		if cb := p.eMailPlugin.BeforeModelCallback(); cb != nil {
			if resp, err := cb(ctx, req); err != nil || resp != nil {
				return resp, err
			}
		}
	}
	if p.userNamePlugin != nil {
		if cb := p.userNamePlugin.BeforeModelCallback(); cb != nil {
			if resp, err := cb(ctx, req); err != nil || resp != nil {
				return resp, err
			}
		}
	}
	if p.hostPlugin != nil {
		if cb := p.hostPlugin.BeforeModelCallback(); cb != nil {
			if resp, err := cb(ctx, req); err != nil || resp != nil {
				return resp, err
			}
		}
	}
	return nil, nil
}

func (p *PiiPlugin) AfterModelCallback(ctx agent.Context, resp *model.LLMResponse, err error) (*model.LLMResponse, error) {
	if p.hostPlugin != nil {
		if cb := p.hostPlugin.AfterModelCallback(); cb != nil {
			if r, e := cb(ctx, resp, err); e != nil {
				return nil, e
			} else if r != nil {
				resp = r
			}
		}
	}
	if p.userNamePlugin != nil {
		if cb := p.userNamePlugin.AfterModelCallback(); cb != nil {
			if r, e := cb(ctx, resp, err); e != nil {
				return nil, e
			} else if r != nil {
				resp = r
			}
		}
	}
	if p.eMailPlugin != nil {
		if cb := p.eMailPlugin.AfterModelCallback(); cb != nil {
			if r, e := cb(ctx, resp, err); e != nil {
				return nil, e
			} else if r != nil {
				resp = r
			}
		}
	}
	return resp, nil
}

func (p *PiiPlugin) OnModelErrorCallback(ctx agent.Context, req *model.LLMRequest, err error) (*model.LLMResponse, error) {
	var resp *model.LLMResponse
	if p.eMailPlugin != nil {
		if cb := p.eMailPlugin.OnModelErrorCallback(); cb != nil {
			if r, e := cb(ctx, req, err); e != nil {
				return nil, e
			} else if r != nil {
				resp = r
			}
		}
	}
	if p.userNamePlugin != nil {
		if cb := p.userNamePlugin.OnModelErrorCallback(); cb != nil {
			if r, e := cb(ctx, req, err); e != nil {
				return nil, e
			} else if r != nil {
				resp = r
			}
		}
	}
	if p.hostPlugin != nil {
		if cb := p.hostPlugin.OnModelErrorCallback(); cb != nil {
			if r, e := cb(ctx, req, err); e != nil {
				return nil, e
			} else if r != nil {
				resp = r
			}
		}
	}
	return resp, nil
}
