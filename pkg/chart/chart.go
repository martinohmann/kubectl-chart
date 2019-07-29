package chart

import (
	"fmt"
	"io/ioutil"

	"github.com/imdario/mergo"
	"github.com/martinohmann/kubectl-chart/pkg/hook"
	"github.com/martinohmann/kubectl-chart/pkg/meta"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/renderutil"
	"k8s.io/helm/pkg/timeconv"
)

// Chart is a rendered chart with the config used for rendering, a list of
// chart resources and a map of chart hooks.
type Chart struct {
	Config    *Config
	Resources []runtime.Object
	Hooks     hook.Map
}

// LabelSelector builds valid label selector for a *resource.Builder that
// selects all resources associated to the chart. See doc of const LabelChartName
// for more information.
func LabelSelector(c *Chart) string {
	return fmt.Sprintf("%s=%s", meta.LabelChartName, c.Config.Name)
}

// Config is the configuration for rendering a chart.
type Config struct {
	Dir       string
	Name      string
	Namespace string
	Values    map[interface{}]interface{}
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
func LoadValues(files ...string) (map[interface{}]interface{}, error) {
	values := make(map[interface{}]interface{})

	for _, f := range files {
		buf, err := ioutil.ReadFile(f)
		if err != nil {
			return nil, err
		}

		var v map[interface{}]interface{}

		err = yaml.Unmarshal(buf, &v)
		if err != nil {
			return nil, errors.Wrapf(err, "unmarshal file %s", f)
		}

		err = mergo.Merge(&values, v, mergo.WithOverride)
		if err != nil {
			return nil, errors.Wrapf(err, "merge values from file %s", f)
		}
	}

	return values, nil
}

// ValuesForChart extracts the necessary parts of the values for given chart.
func ValuesForChart(chartName string, values map[interface{}]interface{}) (map[interface{}]interface{}, error) {
	var chartValues map[interface{}]interface{}

	switch cv := values[chartName].(type) {
	case map[interface{}]interface{}:
		chartValues = cv
	case nil:
		chartValues = make(map[interface{}]interface{})
	default:
		return nil, errors.Errorf("values key %q needs to be a map, got %T", chartName, cv)
	}

	if globalValues, ok := values["global"]; ok {
		chartValues["global"] = globalValues
	}

	return chartValues, nil
}

// Include returns true if chartName is included in chartFilter or if
// chartFilter is empty.
func Include(chartFilter []string, chartName string) bool {
	if len(chartFilter) == 0 {
		return true
	}

	for _, name := range chartFilter {
		if name == chartName {
			return true
		}
	}

	return false
}
