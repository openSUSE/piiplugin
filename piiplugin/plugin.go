package piiplugin

import (
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/plugin"
)

type PiiPluginOption func(*PiiPlugin)

type email interface {
	BeforeModelCallback(ctx agent.Context, req *model.LLMRequest) (*model.LLMResponse, error)
	AfterModelCallback(ctx agent.Context, resp *model.LLMResponse, err error) (*model.LLMResponse, error)
	OnModelErrorCallback(ctx agent.Context, req *model.LLMRequest, err error) (*model.LLMResponse, error)
}

type PiiPlugin struct {
	noEMail     bool
	noUserName  bool
	eMailPlugin email
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

func NewPiiPlugin(opts ...PiiPluginOption) *plugin.Plugin {
	p := &PiiPlugin{}
	for _, o := range opts {
		o(p)
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
	return nil, nil
}

func (p *PiiPlugin) AfterModelCallback(ctx agent.Context, resp *model.LLMResponse, err error) (*model.LLMResponse, error) {
	return nil, nil
}

func (p *PiiPlugin) OnModelErrorCallback(ctx agent.Context, req *model.LLMRequest, err error) (*model.LLMResponse, error) {
	return nil, nil
}
