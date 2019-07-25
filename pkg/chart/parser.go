package chart

import (
	"github.com/martinohmann/kubectl-chart/pkg/hook"
	"github.com/martinohmann/kubectl-chart/pkg/yaml"
	"k8s.io/apimachinery/pkg/runtime"
)

type Serializer interface {
	Encode(objs []runtime.Object) ([]byte, error)
	Decode(raw []byte) ([]runtime.Object, error)
}

type Parser struct {
	Serializer Serializer
}

func NewParser(s Serializer) *Parser {
	return &Parser{
		Serializer: s,
	}
}

func NewYAMLParser() *Parser {
	return NewParser(yaml.NewSerializer())
}

func (p *Parser) Parse(raw []byte) ([]runtime.Object, []runtime.Object, error) {
	objs, err := p.Serializer.Decode(raw)
	if err != nil {
		return nil, nil, err
	}

	return p.sort(objs)
}

func (p *Parser) sort(objs []runtime.Object) ([]runtime.Object, []runtime.Object, error) {
	resources := make([]runtime.Object, 0)
	hooks := make([]runtime.Object, 0)

	for _, obj := range objs {
		ok := hasAnnotation(obj, hook.AnnotationHookType)
		if ok {
			hooks = append(hooks, obj)
		} else {
			resources = append(resources, obj)
		}
	}

	return resources, hooks, nil
}
