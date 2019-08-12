package cmd

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	dynamicfakeclient "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
)

type permissiveDryRunVerifier struct{}

func (permissiveDryRunVerifier) HasSupport(gvk schema.GroupVersionKind) error { return nil }

func TestDiffCmd(t *testing.T) {
	cmdtesting.InitTestErrorHandler(t)

	servicePatchCount := 0

	f := newTestFactoryWithFakeDiscovery()
	f.UnstructuredClient = &fake.RESTClient{
		NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			switch p, m := req.URL.Path, req.Method; {
			case p == "/namespaces/test/services/chart1" && m == "GET":
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     cmdtesting.DefaultHeader(),
					Body:       cmdtesting.ObjBody(codec, &corev1.Service{}),
				}, nil
			case p == "/namespaces/test/services/chart1" && m == "PATCH" && servicePatchCount == 0:
				servicePatchCount++
				return &http.Response{
					StatusCode: http.StatusConflict,
					Header:     cmdtesting.DefaultHeader(),
					Body:       cmdtesting.ObjBody(codec, &corev1.Service{}),
				}, nil
			case p == "/namespaces/test/services/chart1" && m == "PATCH" && servicePatchCount == 1:
				servicePatchCount++
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     cmdtesting.DefaultHeader(),
					Body: cmdtesting.ObjBody(codec, &corev1.Service{Spec: corev1.ServiceSpec{
						Selector: map[string]string{
							"foo": "bar",
						},
					}}),
				}, nil
			case p == "/namespaces/test/statefulsets/chart1" && (m == "GET" || m == "PATCH"):
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Header:     cmdtesting.DefaultHeader(),
					Body:       cmdtesting.ObjBody(codec, &appsv1.StatefulSet{}),
				}, nil
			case p == "/api/v1/namespaces/test" && m == "GET":
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Header:     cmdtesting.DefaultHeader(),
					Body:       cmdtesting.StringBody("{}"),
				}, nil
			case p == "/namespaces/test/statefulsets" && m == "POST":
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     cmdtesting.DefaultHeader(),
					Body:       cmdtesting.ObjBody(codec, &appsv1.StatefulSet{}),
				}, nil
			case p == "/pods" && m == "GET" && req.URL.RawQuery == "labelSelector=kubectl-chart%2Fchart-name%3Dchart1":
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     cmdtesting.DefaultHeader(),
					Body:       cmdtesting.ObjBody(codec, &corev1.Pod{}),
				}, nil
			case m == "GET" && req.URL.RawQuery == "labelSelector=kubectl-chart%2Fchart-name%3Dchart1":
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Header:     cmdtesting.DefaultHeader(),
					Body:       cmdtesting.StringBody("{}"),
				}, nil
			default:
				t.Fatalf("unexpected request: %#v\n%#v", req.URL, req)
				return nil, nil
			}
		}),
	}
	f.ClientConfigVal = cmdtesting.DefaultClientConfig()
	defer f.Cleanup()

	streams, _, buf, _ := genericclioptions.NewTestIOStreams()

	o := NewDiffOptions(streams)

	o.ChartFlags.ChartDir = "../chart/testdata/valid-charts/chart1"
	o.DynamicClient = dynamicfakeclient.NewSimpleDynamicClient(runtime.NewScheme())

	require.NoError(t, o.Complete(f))

	o.DryRunVerifier = &permissiveDryRunVerifier{}

	o.BuilderFactory = func() *resource.Builder {
		return f.NewBuilder()
	}

	require.NoError(t, o.Run())

	expected := `--- apps.v1.StatefulSet.test.chart1
+++ apps.v1.StatefulSet.test.chart1
@@ -1 +1,16 @@
+apiVersion: apps/v1
+kind: StatefulSet
+metadata:
+  creationTimestamp: null
+spec:
+  selector: null
+  serviceName: ""
+  template:
+    metadata:
+      creationTimestamp: null
+    spec:
+      containers: null
+  updateStrategy: {}
+status:
+  replicas: 0
 
--- v1.Service.test.chart1
+++ v1.Service.test.chart1
@@ -1,8 +1,10 @@
 apiVersion: v1
 kind: Service
 metadata:
   creationTimestamp: null
-spec: {}
+spec:
+  selector:
+    foo: bar
 status:
   loadBalancer: {}
 
--- v1.Pod..
+++ <removed>
@@ -1,8 +1 @@
-apiVersion: v1
-kind: Pod
-metadata:
-  creationTimestamp: null
-spec:
-  containers: null
-status: {}
 
`
	assert.Equal(t, expected, buf.String())
}
