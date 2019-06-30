package chart

import (
	"fmt"
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
	// LabelName is used to attach a label to each resource in a rendered chart
	// to be able to keep track of them once they are deployed into a cluster.
	LabelName = "kubectl-chart/name"
)

// Config is the configuration for rendering a chart.
type Config struct {
	Dir       string
	Name      string
	Namespace string
	Values    map[string]interface{}
}

// Render takes a chart config and renders the chart. It returns a map of
// template filepaths and their rendered contents.
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

// LoadValues loads yaml files and stores the contents of provided files into a
// map. The contents are merged left to right, and will overwrite keys present
// in files that are loaded earlier. This makes it possible to layer values
// files.
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

		err = mergo.Merge(&values, v, mergo.WithOverride)
		if err != nil {
			return nil, err
		}
	}
	return nil, nil
}

// AddChartLabel adds a label with the chart name to each obj. See doc of const
// LabelName for more information.
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

// LabelSelector builds valid label selector for a *resource.Builder that
// selects all resources associated to chartName. See doc of const LabelName
// for more information.
func LabelSelector(chartName string) string {
	return fmt.Sprintf("%s=%s", LabelName, chartName)
}
