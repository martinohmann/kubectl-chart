package chart

import (
	"github.com/martinohmann/kubectl-chart/pkg/hook"
	"github.com/martinohmann/kubectl-chart/pkg/meta"
	"github.com/martinohmann/kubectl-chart/pkg/resources"
	"github.com/martinohmann/kubectl-chart/pkg/resources/statefulset"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var statefulSetGK = schema.GroupKind{Group: "apps", Kind: "StatefulSet"}

// templateDecoder decodes rendered chart templates into resources and hooks.
type templateDecoder struct {
	config  *Config
	decoder resources.Decoder
}

// newTemplateDecoder creates a new templateDecoder value with given chart
// config and a decoder for decoding raw templates.
func newTemplateDecoder(config *Config, decoder resources.Decoder) *templateDecoder {
	return &templateDecoder{
		config:  config,
		decoder: decoder,
	}
}

// decodeTemplate accepts raw template bytes and decodes it into chart
// resources and hooks.
func (p *templateDecoder) decodeTemplate(raw []byte) ([]runtime.Object, []*hook.Hook, error) {
	decodedObjs, err := p.decoder.Decode(raw)
	if err != nil {
		return nil, nil, err
	}

	resources := make([]runtime.Object, 0)
	hooks := make([]*hook.Hook, 0)

	for _, obj := range decodedObjs {
		meta.DefaultNamespace(obj, p.config.Namespace)

		if meta.HasAnnotation(obj, meta.AnnotationHookType) {
			h, err := p.prepareHook(obj)
			if err != nil {
				return nil, nil, err
			}

			hooks = append(hooks, h)
		} else {
			err = p.prepareResource(obj)
			if err != nil {
				return nil, nil, err
			}

			resources = append(resources, obj)
		}
	}

	return resources, hooks, nil
}

func (p *templateDecoder) prepareHook(obj runtime.Object) (*hook.Hook, error) {
	h, err := hook.New(obj)
	if err != nil {
		return nil, err
	}

	meta.AddLabel(obj, meta.LabelHookChartName, p.config.Name)
	meta.AddLabel(obj, meta.LabelHookType, h.Type())

	return h, nil
}

// prepareResource prepares a chart resource by setting labels required by
// kubectl-chart to be able to identify resources belonging to a given chart.
func (p *templateDecoder) prepareResource(obj runtime.Object) error {
	meta.AddLabel(obj, meta.LabelChartName, p.config.Name)

	if meta.HasGroupKind(obj, statefulSetGK) {
		return statefulset.AddOwnerLabels(obj)
	}

	return nil
}
