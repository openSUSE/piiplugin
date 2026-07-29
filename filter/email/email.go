package filteremail

import (
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/plugin"
)

type EmailPlugin struct {
	nameParts   map[string]string
	domainParts map[string]string
	keepTLD     bool
}

func NewEmailPlugin() *plugin.Plugin {
	p := EmailPlugin{}
	return plugin.New(plugin.Config{
		Name:                 "eMail_plugin",
		BeforModelCallback:   p.BeforModelCallback,
		AfterModelCallback:   p.AfterModelCallback,
		OnModelErrorCallback: p.OnModelErrorCallback,
	})
}

func (p *EmailPlugin) BeforModelCallback(ctx agent.Contex, req *model.LLMRequest) (*model.LLMResponse, error) {
	return nil, nil
}

func (p *EmailPlugin) AfterModelCallback(ctx agent.Contex, req *model.LLMRequest) (*model.LLMResponse, error) {
	return nil, nil
}

func (p *EmailPlugin) OnModelErrorCallback(ctx agent.Contex, req *model.LLMRequest) (*model.LLMResponse, error) {
	return nil, nil
}
