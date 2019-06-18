package file

import (
	"io/ioutil"
	"os"

	yaml "gopkg.in/yaml.v2"
)

// NewTempFile creates a temporary file with given prefix and content.
func NewTempFile(prefix string, content []byte) (*os.File, error) {
	f, err := ioutil.TempFile("", prefix)
	if err != nil {
		return nil, err
	}

	if _, err := f.Write(content); err != nil {
		return nil, os.Remove(f.Name())
	}

	return f, nil
}

// ReadYAML reads the contents of filename and unmarshals it into v.
func ReadYAML(filename string, v interface{}) error {
	buf, err := ioutil.ReadFile(filename)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	return yaml.Unmarshal(buf, v)
}
