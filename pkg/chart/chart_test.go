package chart

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValuesForChart(t *testing.T) {
	cases := []struct {
		name        string
		values      map[interface{}]interface{}
		expected    map[interface{}]interface{}
		expectError bool
		chartName   string
	}{
		{
			name:      "empty values",
			expected:  map[interface{}]interface{}{},
			chartName: "foo",
		},
		{
			name: "found chart key",
			values: map[interface{}]interface{}{
				"foo": map[interface{}]interface{}{
					"bar": "baz",
				},
				"bar": map[interface{}]interface{}{
					"baz": "qux",
				},
			},
			expected: map[interface{}]interface{}{
				"bar": "baz",
			},
			chartName: "foo",
		},
		{
			name: "found chart key and globals",
			values: map[interface{}]interface{}{
				"global": map[interface{}]interface{}{
					"someglobal": "somevalue",
				},
				"foo": map[interface{}]interface{}{
					"bar": "baz",
				},
				"bar": map[interface{}]interface{}{
					"baz": "qux",
				},
			},
			expected: map[interface{}]interface{}{
				"global": map[interface{}]interface{}{
					"someglobal": "somevalue",
				},
				"bar": "baz",
			},
			chartName: "foo",
		},
		{
			name: "explicit nil",
			values: map[interface{}]interface{}{
				"foo": nil,
			},
			expected:  map[interface{}]interface{}{},
			chartName: "foo",
		},
		{
			name: "type mismatch",
			values: map[interface{}]interface{}{
				"foo": "bar",
			},
			expectError: true,
			chartName:   "foo",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			chartValues, err := ValuesForChart(tc.chartName, tc.values)

			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, chartValues)
			}
		})
	}
}

func TestLoadValues(t *testing.T) {
	cases := []struct {
		name        string
		files       []string
		expected    map[interface{}]interface{}
		expectedErr string
	}{
		{
			name:     "empty file slice",
			expected: map[interface{}]interface{}{},
		},
		{
			name:  "load one file",
			files: []string{"testdata/values.yaml"},
			expected: map[interface{}]interface{}{
				"foo": map[interface{}]interface{}{
					"bar": "baz",
					"qux": "foo",
				},
			},
		},
		{
			name:  "merges two files",
			files: []string{"testdata/values.yaml", "testdata/additional-values.yaml"},
			expected: map[interface{}]interface{}{
				"foo": map[interface{}]interface{}{
					"bar": "qux",
					"baz": "foo",
					"qux": "foo",
				},
				"bar": "baz",
			},
		},
		{
			name:  "merges two files (different order)",
			files: []string{"testdata/additional-values.yaml", "testdata/values.yaml"},
			expected: map[interface{}]interface{}{
				"foo": map[interface{}]interface{}{
					"bar": "baz",
					"baz": "foo",
					"qux": "foo",
				},
				"bar": "baz",
			},
		},
		{
			name:        "returns errors due to non-existent files",
			files:       []string{"testdata/values.yaml", "testdata/non-existent.yaml"},
			expectedErr: "open testdata/non-existent.yaml: no such file or directory",
		},
		{
			name:        "returns errors due to invalid files",
			files:       []string{"testdata/values.yaml", "testdata/invalid-values.yaml"},
			expectedErr: "unmarshal file testdata/invalid-values.yaml: yaml: line 1: did not find expected node content",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			values, err := LoadValues(tc.files...)
			if tc.expectedErr != "" {
				require.Error(t, err)
				assert.Equal(t, tc.expectedErr, err.Error())
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, values)
			}
		})
	}
}
