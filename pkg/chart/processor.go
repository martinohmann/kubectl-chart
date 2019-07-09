package chart

import (
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

// Processor type processes a chart config and renders the contained resources.
// It will also perform post-processing on these resources.
type Processor struct {
	Parser *Parser
}

// NewProcessor creates a new *Processor values which uses given parser to
// parse rendered chart templates.
func NewProcessor(p *Parser) *Processor {
	return &Processor{
		Parser: p,
	}
}

// NewDefaultProcessor creates a new *Processor value.
func NewDefaultProcessor() *Processor {
	return NewProcessor(NewYAMLParser())
}

// Process takes a chart config, renders and processes it. The first return
// value contains all resources found in the rendered chart, whereas the second
// return value contains all chart hooks.
func (p *Processor) Process(config *Config) (*Chart, error) {
	templates, err := Render(config)
	if err != nil {
		return nil, err
	}

	resourceList, hookMap, err := p.parseTemplates(config, templates)
	if err != nil {
		return nil, err
	}

	c := &Chart{
		Config:    config,
		Resources: resourceList,
		Hooks:     hookMap,
	}

	return c, nil
}

func (p *Processor) parseTemplates(config *Config, templates map[string]string) (ResourceList, HookMap, error) {
	resourceList := make(ResourceList, 0)
	hookMap := make(HookMap)

	for name, content := range templates {
		base := filepath.Base(name)
		ext := filepath.Ext(base)

		if strings.HasPrefix(base, "_") || (ext != ".yaml" && ext != ".yml") {
			continue
		}

		resourceObjs, hookObjs, err := p.Parser.Parse([]byte(content))
		if err != nil {
			return nil, nil, errors.Wrapf(err, "while parsing template %q", name)
		}

		err = LabelStatefulSets(resourceObjs)
		if err != nil {
			return nil, nil, err
		}

		for _, obj := range resourceObjs {
			r := NewResource(obj)
			r.SetLabel(LabelChartName, config.Name)
			r.DefaultNamespace(config.Namespace)

			resourceList = append(resourceList, r)
		}

		for _, obj := range hookObjs {
			h := NewHook(obj)

			if err := ValidateHook(h); err != nil {
				return nil, nil, err
			}

			h.SetLabel(LabelHookChartName, config.Name)
			h.SetLabel(LabelHookType, h.Type())
			h.DefaultNamespace(config.Namespace)

			if hookMap[h.Type()] == nil {
				hookMap[h.Type()] = HookList{h}
			} else {
				hookMap[h.Type()] = append(hookMap[h.Type()], h)
			}
		}
	}

	SortResources(resourceList, ApplyOrder)

	return resourceList, hookMap, nil
}
