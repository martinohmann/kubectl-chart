package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func TestDumpValuesCmd(t *testing.T) {
	streams, _, buf, _ := genericclioptions.NewTestIOStreams()

	cmd := NewDumpValuesCmd(streams)

	cmd.SetArgs([]string{"-f", "../chart/testdata/valid-charts/chart1"})

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
	streams, _, buf, _ := genericclioptions.NewTestIOStreams()

	cmd := NewDumpValuesCmd(streams)

	cmd.SetArgs([]string{"-f", "../chart/testdata/valid-charts", "-R"})

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
