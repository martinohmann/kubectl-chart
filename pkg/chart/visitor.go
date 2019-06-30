package chart

import (
	"io/ioutil"
	"path/filepath"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
)

type Visitor struct {
	Processor   *Processor
	ValueFiles  []string
	ChartDir    string
	ChartFilter []string
	Namespace   string
	Recursive   bool
}

func (v *Visitor) Visit(fn func(config *Config, resources, hooks []runtime.Object, err error) error) error {
	values, err := LoadValues(v.ValueFiles...)
	if err != nil {
		return err
	}

	configs, err := v.buildChartConfigs(values)
	if err != nil {
		return err
	}

	for _, config := range configs {
		if len(v.ChartFilter) > 0 && !contains(v.ChartFilter, config.Name) {
			continue
		}

		resources, hooks, err := v.Processor.Process(config)
		if err != nil {
			return errors.Wrapf(err, "while processing chart %q", config.Name)
		}

		if err != nil {
			if fnErr := fn(config, resources, hooks, err); fnErr != nil {
				return fnErr
			}
			continue
		}

		if err := fn(config, resources, hooks, nil); err != nil {
			return err
		}
	}

	return err
}

func (v *Visitor) buildChartConfigs(values map[string]interface{}) ([]*Config, error) {
	configs := make([]*Config, 0)

	if v.Recursive {
		infos, err := ioutil.ReadDir(v.ChartDir)
		if err != nil {
			return nil, err
		}

		for _, info := range infos {
			if !info.IsDir() {
				continue
			}

			configs = append(configs, &Config{
				Dir:       filepath.Join(v.ChartDir, info.Name()),
				Name:      info.Name(),
				Namespace: v.Namespace,
				Values:    values,
			})
		}
	} else {
		configs = append(configs, &Config{
			Dir:       v.ChartDir,
			Name:      filepath.Base(v.ChartDir),
			Namespace: v.Namespace,
			Values:    values,
		})
	}

	return configs, nil
}

func contains(s []string, v string) bool {
	for _, u := range s {
		if u == v {
			return true
		}
	}

	return false
}
