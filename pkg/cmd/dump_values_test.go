package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
)

func TestDumpValuesCmd(t *testing.T) {
	cmdtesting.InitTestErrorHandler(t)

	streams, _, buf, _ := genericclioptions.NewTestIOStreams()

	cmd := NewDumpValuesCmd(streams)

	cmd.Flags().Set("chart-dir", "../chart/testdata/valid-charts/chart1")

	err := cmd.Execute()

	require.NoError(t, err)

	expected := `---
# Merged values for chart: chart1
---
affinity: {}
fullnameOverride: ""
hookType: post-apply
image:
  pullPolicy: IfNotPresent
  repository: nginx
  tag: stable
ingress:
  annotations: {}
  enabled: false
  hosts:
  - host: chart-example.local
    paths: []
  tls: []
nameOverride: ""
nodeSelector: {}
replicaCount: 1
resources: {}
service:
  port: 80
  type: ClusterIP
tolerations: []
`

	assert.Equal(t, expected, buf.String())
}

func TestDumpValuesCmd_Recursive(t *testing.T) {
	cmdtesting.InitTestErrorHandler(t)

	streams, _, buf, _ := genericclioptions.NewTestIOStreams()

	cmd := NewDumpValuesCmd(streams)

	cmd.Flags().Set("chart-dir", "../chart/testdata/valid-charts")
	cmd.Flags().Set("recursive", "true")

	err := cmd.Execute()

	require.NoError(t, err)

	expected := `---
# Merged values for chart: chart1
---
affinity: {}
fullnameOverride: ""
hookType: post-apply
image:
  pullPolicy: IfNotPresent
  repository: nginx
  tag: stable
ingress:
  annotations: {}
  enabled: false
  hosts:
  - host: chart-example.local
    paths: []
  tls: []
nameOverride: ""
nodeSelector: {}
replicaCount: 1
resources: {}
service:
  port: 80
  type: ClusterIP
tolerations: []
---
# Merged values for chart: chart2
---
affinity: {}
fullnameOverride: ""
image:
  pullPolicy: IfNotPresent
  repository: nginx
  tag: stable
nameOverride: ""
nodeSelector: {}
replicaCount: 1
resources: {}
service:
  port: 80
  type: ClusterIP
tolerations: []
`

	assert.Equal(t, expected, buf.String())
}

func TestDumpValuesCmd_Filter(t *testing.T) {
	cmdtesting.InitTestErrorHandler(t)

	streams, _, buf, _ := genericclioptions.NewTestIOStreams()

	cmd := NewDumpValuesCmd(streams)

	cmd.Flags().Set("chart-dir", "../chart/testdata/valid-charts")
	cmd.Flags().Set("recursive", "true")
	cmd.Flags().Set("chart-filter", "chart2")

	err := cmd.Execute()

	require.NoError(t, err)

	expected := `---
# Merged values for chart: chart2
---
affinity: {}
fullnameOverride: ""
image:
  pullPolicy: IfNotPresent
  repository: nginx
  tag: stable
nameOverride: ""
nodeSelector: {}
replicaCount: 1
resources: {}
service:
  port: 80
  type: ClusterIP
tolerations: []
`

	assert.Equal(t, expected, buf.String())
}

func TestDumpValuesOptions_Error(t *testing.T) {
	streams, _, buf, _ := genericclioptions.NewTestIOStreams()

	o := NewDumpValuesOptions(streams)
	o.ChartFlags.ChartDir = "../chart/testdata"

	require.NoError(t, o.Complete())

	err := o.Run()

	_ = buf

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no Chart.yaml exists in directory")
}
