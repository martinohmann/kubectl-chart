package cmd

import (
	"errors"
	"testing"

	"github.com/martinohmann/kubectl-chart/pkg/hook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
)

func TestRenderCmd(t *testing.T) {
	cmdtesting.InitTestErrorHandler(t)

	f := cmdtesting.NewTestFactory().WithNamespace("test")
	streams, _, buf, _ := genericclioptions.NewTestIOStreams()

	cmd := NewRenderCmd(f, streams)

	cmd.Flags().Set("chart-dir", "../chart/testdata/valid-charts")
	cmd.Flags().Set("recursive", "true")

	require.NoError(t, cmd.Execute())

	expected := `---
apiVersion: v1
kind: Service
metadata:
  labels:
    app.kubernetes.io/instance: chart1
    app.kubernetes.io/managed-by: Tiller
    app.kubernetes.io/name: chart1
    helm.sh/chart: chart1-0.1.0
    kubectl-chart/chart-name: chart1
  name: chart1
  namespace: test
spec:
  ports:
  - name: http
    port: 80
    protocol: TCP
    targetPort: http
  selector:
    app.kubernetes.io/instance: chart1
    app.kubernetes.io/name: chart1
  type: ClusterIP
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  labels:
    kubectl-chart/chart-name: chart1
  name: chart1
  namespace: test
spec:
  selector:
    matchLabels:
      foo: bar
      kubectl-chart/owned-by-statefulset: chart1
  serviceName: chart1
  template:
    metadata:
      labels:
        foo: bar
        kubectl-chart/owned-by-statefulset: chart1
  volumeClaimTemplates:
  - metadata:
      labels:
        kubectl-chart/owned-by-statefulset: chart1
      name: baz
---
apiVersion: v1
kind: Service
metadata:
  labels:
    app.kubernetes.io/instance: chart2
    app.kubernetes.io/managed-by: Tiller
    app.kubernetes.io/name: chart2
    helm.sh/chart: chart2-0.1.0
    kubectl-chart/chart-name: chart2
  name: chart2
  namespace: test
spec:
  ports:
  - name: http
    port: 80
    protocol: TCP
    targetPort: http
  selector:
    app.kubernetes.io/instance: chart2
    app.kubernetes.io/name: chart2
  type: ClusterIP
`

	assert.Equal(t, expected, buf.String())
}

func TestRenderCmd_Validate(t *testing.T) {
	tests := []struct {
		name        string
		hookType    string
		expectedErr string
	}{
		{
			name: "empty hook type",
		},
		{
			name:     "all hook types",
			hookType: "all",
		},
		{
			name:        "invalid hook type",
			hookType:    "invalid-hook-type",
			expectedErr: `unsupported hook type "invalid-hook-type", allowed values are: [pre-apply post-apply pre-delete post-delete]`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			o := NewRenderOptions(genericclioptions.NewTestIOStreamsDiscard())

			o.HookType = test.hookType

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

func TestRenderCmd_HookTypes(t *testing.T) {
	tests := []struct {
		name     string
		hookType string
		expected string
	}{
		{
			name: "render resources",
			expected: `---
apiVersion: v1
kind: Service
metadata:
  labels:
    app.kubernetes.io/instance: chart1
    app.kubernetes.io/managed-by: Tiller
    app.kubernetes.io/name: chart1
    helm.sh/chart: chart1-0.1.0
    kubectl-chart/chart-name: chart1
  name: chart1
  namespace: test
spec:
  ports:
  - name: http
    port: 80
    protocol: TCP
    targetPort: http
  selector:
    app.kubernetes.io/instance: chart1
    app.kubernetes.io/name: chart1
  type: ClusterIP
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  labels:
    kubectl-chart/chart-name: chart1
  name: chart1
  namespace: test
spec:
  selector:
    matchLabels:
      foo: bar
      kubectl-chart/owned-by-statefulset: chart1
  serviceName: chart1
  template:
    metadata:
      labels:
        foo: bar
        kubectl-chart/owned-by-statefulset: chart1
  volumeClaimTemplates:
  - metadata:
      labels:
        kubectl-chart/owned-by-statefulset: chart1
      name: baz
`,
		},
		{
			name:     "render all hooks",
			hookType: "all",
			expected: `---
apiVersion: batch/v1
kind: Job
metadata:
  annotations:
    kubectl-chart/hook-type: post-apply
  labels:
    app.kubernetes.io/instance: chart1
    app.kubernetes.io/managed-by: Tiller
    app.kubernetes.io/name: chart1
    helm.sh/chart: chart1-0.1.0
    kubectl-chart/hook-chart-name: chart1
    kubectl-chart/hook-type: post-apply
  name: chart1
  namespace: bar
spec:
  template:
    spec:
      containers:
      - image: nginx:stable
        imagePullPolicy: IfNotPresent
        name: chart1
        ports:
        - containerPort: 80
          name: http
          protocol: TCP
      restartPolicy: Never
`,
		},
		{
			name:     "render pre-apply hooks",
			hookType: hook.PreApply,
		},
		{
			name:     "render post-apply hooks",
			hookType: hook.PostApply,
			expected: `---
apiVersion: batch/v1
kind: Job
metadata:
  annotations:
    kubectl-chart/hook-type: post-apply
  labels:
    app.kubernetes.io/instance: chart1
    app.kubernetes.io/managed-by: Tiller
    app.kubernetes.io/name: chart1
    helm.sh/chart: chart1-0.1.0
    kubectl-chart/hook-chart-name: chart1
    kubectl-chart/hook-type: post-apply
  name: chart1
  namespace: bar
spec:
  template:
    spec:
      containers:
      - image: nginx:stable
        imagePullPolicy: IfNotPresent
        name: chart1
        ports:
        - containerPort: 80
          name: http
          protocol: TCP
      restartPolicy: Never
`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			streams, _, buf, _ := genericclioptions.NewTestIOStreams()
			o := NewRenderOptions(streams)

			o.ChartFlags.ChartDir = "../chart/testdata/valid-charts/chart1"
			o.Visitor, _ = o.ChartFlags.ToVisitor("test")
			o.HookType = test.hookType

			require.NoError(t, o.Run())
			assert.Equal(t, test.expected, buf.String())
		})
	}
}

type badEncoder struct{}

func (badEncoder) Encode([]runtime.Object) ([]byte, error) {
	return nil, errors.New("meeh")
}

func TestRenderCmd_EncodeError(t *testing.T) {
	o := &RenderOptions{
		IOStreams: genericclioptions.NewTestIOStreamsDiscard(),
		Encoder:   &badEncoder{},
	}

	o.ChartFlags.ChartDir = "../chart/testdata/valid-charts/chart1"
	o.Visitor, _ = o.ChartFlags.ToVisitor("test")

	err := o.Run()

	require.Error(t, err)
	assert.Equal(t, "meeh", err.Error())
}
