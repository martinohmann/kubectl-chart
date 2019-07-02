package chart

import (
	"path/filepath"
	"strings"

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
func (p *Processor) Process(config *Config) ([]runtime.Object, []runtime.Object, error) {
	templates, err := Render(config)
	if err != nil {
		return nil, nil, err
	}

	resources, hookResources, err := p.parseTemplates(templates)
	if err != nil {
		return nil, nil, err
	}

	err = postProcessObjects(config, resources...)
	if err != nil {
		return nil, nil, err
	}

	err = postProcessObjects(config, hookResources...)
	if err != nil {
		return nil, nil, err
	}

	hooks := make(map[string][]*Hook)

	for _, obj := range hookResources {
		hook, err := ParseHook(obj)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "failed to parse hook in chart %q", config.Name)
		}

		if hooks[hook.Type] == nil {
			hooks[hook.Type] = make([]*Hook, 0, 1)
		}

		hooks[hook.Type] = append(hooks[hook.Type], hook)
	}

	//@TODO(mohmann): use hooks

	return resources, hookResources, nil
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

func postProcessObjects(config *Config, objs ...runtime.Object) error {
	err := resources.EnsureNamespaceSet(config.Namespace, objs...)
	if err != nil {
		return err
	}

	err = AddChartLabel(config.Name, objs...)
	if err != nil {
		return err
	}

	return resources.LabelStatefulSets(objs)
}
