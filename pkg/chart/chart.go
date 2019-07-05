package chart

import (
	"fmt"
	"io/ioutil"

	"github.com/imdario/mergo"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/renderutil"
	"k8s.io/helm/pkg/timeconv"
)

// Chart is a rendered chart with the config used for rendering, a list of
// chart resources and a map of chart hooks.
type Chart struct {
	Config    *Config
	Resources ResourceList
	Hooks     HookMap
}

// LabelSelector builds valid label selector for a *resource.Builder that
// selects all resources associated to the chart. See doc of const LabelChartName
// for more information.
func (c *Chart) LabelSelector() string {
	return fmt.Sprintf("%s=%s", LabelChartName, c.Config.Name)
}

func (c *Chart) HookLabelSelector(hookType string) string {
	return fmt.Sprintf("%s=%s,%s=%s", LabelHookChartName, c.Config.Name, LabelHookType, hookType)
}

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

	return values, nil
}

// valuesForChart extracts the necessary parts of the values for given chart.
func valuesForChart(chartName string, values map[string]interface{}) (map[string]interface{}, error) {
	var chartValues map[string]interface{}

	switch cv := values[chartName].(type) {
	case map[string]interface{}:
		chartValues = cv
	case nil:
		chartValues = make(map[string]interface{})
	default:
		return nil, errors.Errorf("values key %q needs to be a map, got %T", chartName, cv)
	}

	if globalValues, ok := values["global"]; ok {
		chartValues["global"] = globalValues
	}

	return chartValues, nil
}
