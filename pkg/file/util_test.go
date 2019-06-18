package file

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testStruct struct {
	Foo string
	Bar int
}

func TestReadYAML(t *testing.T) {
	content := []byte("---\nfoo: bar\nbar: 2\n")
	f, err := NewTempFile("foo.yaml", content)
	if !assert.NoError(t, err) {
		return
	}

	defer os.Remove(f.Name())

	v := &testStruct{}

	if !assert.NoError(t, ReadYAML(f.Name(), v)) {
		return
	}

	assert.Equal(t, "bar", v.Foo)
	assert.Equal(t, 2, v.Bar)
}
