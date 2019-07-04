package chart

import (
	"io/ioutil"
	"path/filepath"

	"github.com/pkg/errors"
)

// VisitorOptions configure the charts the visitor should visit.
type VisitorOptions struct {
	ValueFiles  []string
	ChartDir    string
	ChartFilter []string
	Namespace   string
	Recursive   bool
}

// VisitorFunc is the signature of a function that is called for every chart
// that is encountered by the visitor.
type VisitorFunc func(chart *Chart, err error) error

// Visitor is a chart visitor.
type Visitor struct {
	Processor *Processor
	Options   VisitorOptions
}

// NewVisitor creates a new *Visitor which uses given *Processor and
// VisitorOptions to process charts.
func NewVisitor(p *Processor, o VisitorOptions) *Visitor {
	return &Visitor{
		Processor: p,
		Options:   o,
	}
}

// Visit accepts a function that is called for every chart the visitor
// encounters. The visitor will use a chart processor to process every chart
// before passing the chart config, resources and hooks to fn.
func (v *Visitor) Visit(fn VisitorFunc) error {
	values, err := LoadValues(v.Options.ValueFiles...)
	if err != nil {
		return err
	}

	configs, err := v.buildChartConfigs(values)
	if err != nil {
		return err
	}

	for _, config := range configs {
		if !v.includeChart(config.Name) {
			continue
		}

		c, err := v.Processor.Process(config)
		if err != nil {
			return errors.Wrapf(err, "while processing chart %q", config.Name)
		}

		if err != nil {
			if fnErr := fn(c, err); fnErr != nil {
				return fnErr
			}
			continue
		}

		if err := fn(c, nil); err != nil {
			return err
		}
	}

	return err
}

func (v *Visitor) buildChartConfigs(values map[string]interface{}) ([]*Config, error) {
	configs := make([]*Config, 0)

	if v.Options.Recursive {
		infos, err := ioutil.ReadDir(v.Options.ChartDir)
		if err != nil {
			return nil, err
		}

		for _, info := range infos {
			if !info.IsDir() {
				continue
			}

			configs = append(configs, &Config{
				Dir:       filepath.Join(v.Options.ChartDir, info.Name()),
				Name:      info.Name(),
				Namespace: v.Options.Namespace,
				Values:    values,
			})
		}
	} else {
		configs = append(configs, &Config{
			Dir:       v.Options.ChartDir,
			Name:      filepath.Base(v.Options.ChartDir),
			Namespace: v.Options.Namespace,
			Values:    values,
		})
	}

	return configs, nil
}

func (v *Visitor) includeChart(chartName string) bool {
	if len(v.Options.ChartFilter) == 0 {
		return true
	}

	for _, name := range v.Options.ChartFilter {
		if name == chartName {
			return true
		}
	}

	return false
}
