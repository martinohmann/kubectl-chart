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

// Visitor is a type that visits charts.
type Visitor interface {
	// Visit accepts a function that is called for every chart the visitor
	// encounters.
	Visit(fn VisitorFunc) error
}

// Visitor is a chart visitor.
type visitor struct {
	Processor *Processor
	Options   VisitorOptions
}

// NewVisitor creates a new *Visitor which uses given *Processor and
// VisitorOptions to process charts.
func NewVisitor(p *Processor, o VisitorOptions) Visitor {
	return &visitor{
		Processor: p,
		Options:   o,
	}
}

// Visit implements Visitor. The visitor will use a chart processor to process
// every chart before passing the chart config, resources and hooks to fn.
func (v *visitor) Visit(fn VisitorFunc) error {
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

func (v *visitor) buildChartConfigs(values map[interface{}]interface{}) ([]*Config, error) {
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

			chartName := info.Name()

			chartValues, err := ValuesForChart(chartName, values)
			if err != nil {
				return nil, err
			}

			configs = append(configs, &Config{
				Dir:       filepath.Join(v.Options.ChartDir, chartName),
				Name:      chartName,
				Namespace: v.Options.Namespace,
				Values:    chartValues,
			})
		}
	} else {
		chartName := filepath.Base(v.Options.ChartDir)

		chartValues, err := ValuesForChart(chartName, values)
		if err != nil {
			return nil, err
		}

		configs = append(configs, &Config{
			Dir:       v.Options.ChartDir,
			Name:      chartName,
			Namespace: v.Options.Namespace,
			Values:    chartValues,
		})
	}

	return configs, nil
}

func (v *visitor) includeChart(chartName string) bool {
	return Include(v.Options.ChartFilter, chartName)
}

// ReverseVisitor wraps a Visitor and visits all charts in the reverse order.
type ReverseVisitor struct {
	Visitor Visitor
}

// NewReverseVisitor creates a new *ReverseVisitor which wraps visitor.
func NewReverseVisitor(visitor Visitor) *ReverseVisitor {
	return &ReverseVisitor{
		Visitor: visitor,
	}
}

// Visit implements Visitor.
func (v *ReverseVisitor) Visit(fn VisitorFunc) error {
	charts := make([]*Chart, 0)

	err := v.Visitor.Visit(func(c *Chart, err error) error {
		if err != nil {
			return err
		}

		charts = append(charts, c)

		return nil
	})

	for i := len(charts) - 1; i >= 0; i-- {
		if err != nil {
			if fnErr := fn(charts[i], err); fnErr != nil {
				return fnErr
			}
			continue
		}

		if err := fn(charts[i], nil); err != nil {
			return err
		}
	}

	return err
}
