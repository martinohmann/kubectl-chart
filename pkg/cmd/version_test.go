package cmd

import (
	"fmt"
	"testing"

	"github.com/martinohmann/kubectl-chart/pkg/version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
)

func TestVersionCmd(t *testing.T) {
	cmdtesting.InitTestErrorHandler(t)

	gitVersion := version.Get().GitVersion

	tests := []struct {
		name     string
		output   string
		short    bool
		expected string
	}{
		{
			name:     "default output",
			expected: fmt.Sprintf(`&version.Info{GitVersion:"%s", `, gitVersion),
		},
		{
			name:     "short output",
			short:    true,
			expected: fmt.Sprintf("%s\n", gitVersion),
		},
		{
			name:     "json output",
			output:   "json",
			expected: fmt.Sprintf(`{"gitVersion":"%s",`, gitVersion),
		},
		{
			name:     "yaml output",
			output:   "yaml",
			expected: fmt.Sprintf("gitVersion: %s\n", gitVersion),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			streams, _, buf, _ := genericclioptions.NewTestIOStreams()

			cmd := NewVersionCmd(streams)

			cmd.Flags().Set("short", fmt.Sprintf("%v", test.short))
			cmd.Flags().Set("output", test.output)

			err := cmd.Execute()

			require.NoError(t, err)

			assert.Contains(t, buf.String(), test.expected)
		})
	}
}

func TestVersionCmd_Validate(t *testing.T) {
	tests := []struct {
		name        string
		short       bool
		output      string
		expectedErr string
	}{
		{
			name: "empty output format",
		},
		{
			name:        "invalid output format",
			output:      "xml",
			expectedErr: ErrInvalidOutputFormat.Error(),
		},
		{
			name:   "valid output format",
			output: "yaml",
		},
		{
			name:        "invalid flag combination",
			short:       true,
			output:      "yaml",
			expectedErr: ErrIllegalVersionFlagCombination.Error(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			o := NewVersionOptions(genericclioptions.NewTestIOStreamsDiscard())

			o.Short = test.short
			o.Output = test.output

			err := o.Validate()

			if test.expectedErr != "" {
				require.Error(t, err)
				assert.Equal(t, test.expectedErr, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
