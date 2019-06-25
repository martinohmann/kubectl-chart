package chart

import (
	"io/ioutil"

	"github.com/imdario/mergo"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/renderutil"
	"k8s.io/helm/pkg/timeconv"
)

const (
	LabelName = "kubectl-chart.io/name"
)

type Config struct {
	Dir       string
	Name      string
	Namespace string
	Values    map[string]interface{}
}

func Render(config *Config) (map[string]string, error) {
	c, err := chartutil.Load(config.Dir)
	if err != nil {
		return nil, err
	}

	rawVals, err := yaml.Marshal(config.Values)
	if err != nil {
		return nil, err
	}

	chartConfig := &chart.Config{
		Raw:    string(rawVals),
		Values: map[string]*chart.Value{},
	}

	renderOptions := renderutil.Options{
		ReleaseOptions: chartutil.ReleaseOptions{
			Name:      config.Name,
			Namespace: config.Namespace,
			Time:      timeconv.Now(),
		},
	}

	return renderutil.Render(c, chartConfig, renderOptions)
}

func LoadValues(files ...string) (map[string]interface{}, error) {
	values := make(map[string]interface{})

	for _, f := range files {
		buf, err := ioutil.ReadFile(f)
		if err != nil {
			return nil, err
		}

		var v map[string]interface{}

		err = yaml.Unmarshal(buf, &v)
		if err != nil {
			return nil, err
		}

		err = mergo.Merge(&values, v)
		if err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func AddChartLabel(name string, objs ...runtime.Object) error {
	for _, obj := range objs {
		u, ok := obj.(*unstructured.Unstructured)
		if !ok {
			return errors.Errorf("failed to add chart label %q due to illegal object type: %T", name, obj)
		}

		err := unstructured.SetNestedField(u.Object, name, "metadata", "labels", LabelName)
		if err != nil {
			return errors.Wrapf(err, "failed to add chart label %q", name)
		}
	}

	return nil
}
