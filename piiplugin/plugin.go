package piiplugin

import (
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/plugin"
)

type PiiPluginOption func(*PiiPlugin)

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
	return plugin.New(plugin.Config{
		Name:                 "pii_plugin",
		BeforModelCallback:   p.BeforModelCallback,
		AfterModelCallback:   p.AfterModelCallback,
		OnModelErrorCallback: p.OnModelErrorCallback,
	})
}

func (p *PiiPlugin) BeforModelCallback(ctx agent.Contex, req *model.LLMRequest) (*model.LLMResponse, error) {
	return nil, nil
}

func (p *PiiPlugin) AfterModelCallback(ctx agent.Contex, req *model.LLMRequest) (*model.LLMResponse, error) {
	return nil, nil
}

func (p *PiiPlugin) OnModelErrorCallback(ctx agent.Contex, req *model.LLMRequest) (*model.LLMResponse, error) {
	return nil, nil
}
