package chart

import (
	"path/filepath"
	"strings"

	"github.com/martinohmann/kubectl-chart/pkg/hook"
	"github.com/martinohmann/kubectl-chart/pkg/resources"
	"github.com/martinohmann/kubectl-chart/pkg/yaml"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
)

// Processor type processes a chart config and renders the contained resources.
// It will also perform post-processing on these resources.
type Processor struct {
	Decoder resources.Decoder
}

// NewProcessor creates a new *Processor values which uses given decoder to
// decode rendered chart templates.
func NewProcessor(d resources.Decoder) *Processor {
	return &Processor{
		Decoder: d,
	}
}

// NewDefaultProcessor creates a new *Processor value.
func NewDefaultProcessor() *Processor {
	return NewProcessor(yaml.NewSerializer())
}

// Process takes a chart config, renders and processes it.
func (p *Processor) Process(config *Config) (*Chart, error) {
	templates, err := Render(config)
	if err != nil {
		return nil, err
	}

	resources, hookMap, err := p.decodeTemplates(config, templates)
	if err != nil {
		return nil, err
	}

	c := &Chart{
		Config:    config,
		Resources: resources,
		Hooks:     hookMap,
	}

	return c, nil
}

// decodeTemplates decodes templates into resources and hooks for given chart
// config.
func (p *Processor) decodeTemplates(config *Config, templates map[string]string) ([]runtime.Object, hook.Map, error) {
	objs := make([]runtime.Object, 0)
	hookMap := make(hook.Map)

	decoder := newTemplateDecoder(config, p.Decoder)

	for name, content := range templates {
		base := filepath.Base(name)
		ext := filepath.Ext(base)

		if strings.HasPrefix(base, "_") || (ext != ".yaml" && ext != ".yml") {
			continue
		}

		resources, hooks, err := decoder.decodeTemplate([]byte(content))
		if err != nil {
			return nil, nil, errors.Wrapf(err, "while parsing template %q", name)

		}

		objs = append(objs, resources...)
		hookMap.Add(hooks...)
	}

	resources.SortByKind(objs, resources.ApplyOrder)

	return objs, hookMap, nil
}
