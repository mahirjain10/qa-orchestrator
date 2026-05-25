package llm

import "context"

type BaseProvider struct {
	config *Config
	name   string
}

func (p *BaseProvider) Name() string {
	return p.name
}

func (p *BaseProvider) CheckContext(ctx context.Context) error {
	return checkContext(ctx)
}

func (p *BaseProvider) Config() *Config {
	return p.config
}

func (p *BaseProvider) ValidateModel(model string) error {
	return validateModel(model, p.name)
}

func (p *BaseProvider) ApplyConfig(cfg *Config) {
	p.config = cfg
}
