package chart

import (
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
)

type Processor struct {
	Parser *Parser
}

func NewProcessor(p *Parser) *Processor {
	return &Processor{
		Parser: p,
	}
}

func NewDefaultProcessor() *Processor {
	return NewProcessor(NewYAMLParser())
}

func (p *Processor) Process(config *Config) ([]runtime.Object, []runtime.Object, error) {
	templates, err := Render(config)
	if err != nil {
		return nil, nil, err
	}

	resources, hooks, err := p.parseTemplates(templates)
	if err != nil {
		return nil, nil, err
	}

	err = AddChartLabel(config.Name, resources...)
	if err != nil {
		return nil, nil, err
	}

	err = AddChartLabel(config.Name, hooks...)
	if err != nil {
		return nil, nil, err
	}

	return resources, hooks, nil
}

func (p *Processor) parseTemplates(templates map[string]string) ([]runtime.Object, []runtime.Object, error) {
	resources := make([]runtime.Object, 0)
	hooks := make([]runtime.Object, 0)

	for name, content := range templates {
		base := filepath.Base(name)
		ext := filepath.Ext(base)

		if strings.HasPrefix(base, "_") || (ext != ".yaml" && ext != ".yml") {
			continue
		}

		r, h, err := p.Parser.Parse([]byte(content))
		if err != nil {
			return nil, nil, err
		}

		resources = append(resources, r...)
		hooks = append(hooks, h...)
	}

	return resources, hooks, nil
}
