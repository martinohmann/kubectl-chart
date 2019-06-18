package template

import "github.com/martinohmann/kubectl-chart/pkg/file"

type ValueLoader interface {
	LoadValues() (map[string]interface{}, error)
}

type YAMLValueLoader struct {
	Filename string
}

func NewYAMLValueLoader(filename string) *YAMLValueLoader {
	return &YAMLValueLoader{
		Filename: filename,
	}
}

func (l *YAMLValueLoader) LoadValues() (map[string]interface{}, error) {
	var v map[string]interface{}

	err := file.ReadYAML(l.Filename, &v)

	return v, err
}
