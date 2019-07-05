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
			chartValues, err := valuesForChart(tc.chartName, tc.values)

			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, chartValues)
			}
		})
	}
}
