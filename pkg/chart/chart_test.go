package chart

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValuesForChart(t *testing.T) {
	cases := []struct {
		name        string
		values      map[string]interface{}
		expected    map[string]interface{}
		expectError bool
		chartName   string
	}{
		{
			name:      "empty values",
			expected:  map[string]interface{}{},
			chartName: "foo",
		},
		{
			name: "found chart key",
			values: map[string]interface{}{
				"foo": map[string]interface{}{
					"bar": "baz",
				},
				"bar": map[string]interface{}{
					"baz": "qux",
				},
			},
			expected: map[string]interface{}{
				"bar": "baz",
			},
			chartName: "foo",
		},
		{
			name: "found chart key and globals",
			values: map[string]interface{}{
				"global": map[string]interface{}{
					"someglobal": "somevalue",
				},
				"foo": map[string]interface{}{
					"bar": "baz",
				},
				"bar": map[string]interface{}{
					"baz": "qux",
				},
			},
			expected: map[string]interface{}{
				"global": map[string]interface{}{
					"someglobal": "somevalue",
				},
				"bar": "baz",
			},
			chartName: "foo",
		},
		{
			name: "explicit nil",
			values: map[string]interface{}{
				"foo": nil,
			},
			expected:  map[string]interface{}{},
			chartName: "foo",
		},
		{
			name: "type mismatch",
			values: map[string]interface{}{
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
