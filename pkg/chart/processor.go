package chart

import (
	"path/filepath"
	"strings"

	"github.com/martinohmann/kubectl-chart/pkg/hook"
	"github.com/martinohmann/kubectl-chart/pkg/resources"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
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

func (p *Processor) parseTemplates(config *Config, templates map[string]string) ([]runtime.Object, hook.Map, error) {
	resourceList := make([]runtime.Object, 0)
	hookMap := make(hook.Map)

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

		for _, obj := range resourceObjs {
			defaultNamespace(obj, config.Namespace)
			setLabel(obj, LabelChartName, config.Name)

			gvk := obj.GetObjectKind().GroupVersionKind()

			if gvk.GroupKind() == statefulSetGK {
				err = labelStatefulSet(obj)
				if err != nil {
					return nil, nil, err
				}
			}

			resourceList = append(resourceList, obj)
		}

		for _, obj := range hookObjs {
			h, err := hook.New(obj)
			if err != nil {
				return nil, nil, err
			}

			setLabel(obj, hook.LabelHookChartName, config.Name)
			setLabel(obj, hook.LabelHookType, h.Type())
			defaultNamespace(obj, config.Namespace)

			hookMap.Add(h)
		}
	}

	resources.SortByKind(resourceList, resources.ApplyOrder)

	return resourceList, hookMap, nil
}
