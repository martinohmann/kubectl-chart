package cmd

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/discovery"
	fakediscovery "k8s.io/client-go/discovery/fake"
	dynamicfakeclient "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/rest/fake"
	clienttesting "k8s.io/client-go/testing"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
	"k8s.io/kubectl/pkg/scheme"
)

var codec = scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)

type fakeCachedDiscovery struct {
	*fakediscovery.FakeDiscovery
}

func (f *fakeCachedDiscovery) Fresh() bool { return false }
func (f *fakeCachedDiscovery) Invalidate() {}

type testFactoryWithFakeDiscovery struct {
	*cmdtesting.TestFactory
	FakeDiscovery *fakediscovery.FakeDiscovery
}

func (f *testFactoryWithFakeDiscovery) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	if f.FakeDiscovery != nil {
		return &fakeCachedDiscovery{f.FakeDiscovery}, nil
	}

	return f.TestFactory.ToDiscoveryClient()
}

func newTestFactoryWithFakeDiscovery(fakeDiscovery *fakediscovery.FakeDiscovery) *testFactoryWithFakeDiscovery {
	if fakeDiscovery == nil {
		fakeDiscovery = &fakediscovery.FakeDiscovery{Fake: &clienttesting.Fake{}}
	}

	return &testFactoryWithFakeDiscovery{
		TestFactory:   cmdtesting.NewTestFactory().WithNamespace("test"),
		FakeDiscovery: fakeDiscovery,
	}
}

func TestApplyCmd(t *testing.T) {
	cmdtesting.InitTestErrorHandler(t)

	f := newTestFactoryWithFakeDiscovery(nil)
	f.UnstructuredClient = &fake.RESTClient{
		NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			switch p, m := req.URL.Path, req.Method; {
			case p == "/namespaces/test/services/chart1" && m == "GET":
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     cmdtesting.DefaultHeader(),
					Body:       cmdtesting.ObjBody(codec, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "chart1"}}),
				}, nil
			case p == "/namespaces/test/services/chart1" && m == "PATCH":
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     cmdtesting.DefaultHeader(),
					Body: cmdtesting.ObjBody(codec, &corev1.Service{
						ObjectMeta: metav1.ObjectMeta{Name: "chart1"},
						Spec: corev1.ServiceSpec{
							Selector: map[string]string{
								"foo": "bar",
							},
						}}),
				}, nil
			case p == "/namespaces/test/statefulsets/chart1" && m == "GET":
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
			default:
				t.Fatalf("unexpected request: %#v\n%#v", req.URL, req)
				return nil, nil
			}
		}),
	}
	f.ClientConfigVal = cmdtesting.DefaultClientConfig()
	defer f.Cleanup()

	streams, _, buf, _ := genericclioptions.NewTestIOStreams()

	o := NewApplyOptions(streams)

	o.DryRun = true
	o.ChartFlags.ChartDir = "../chart/testdata/valid-charts/chart1"
	o.DynamicClientGetter.Client = dynamicfakeclient.NewSimpleDynamicClient(runtime.NewScheme())

	require.NoError(t, o.Complete(f))

	o.BuilderFactory = func() *resource.Builder {
		return f.NewBuilder()
	}

	require.NoError(t, o.Run())

	expected := `service/chart1 configured (dry run)
statefulset.apps/chart1 created (dry run)
job.batch/chart1 triggered (dry run)
`

	assert.Equal(t, expected, buf.String())
}

func TestApplyCmd_Validate(t *testing.T) {
	tests := []struct {
		name         string
		dryRun       bool
		serverDryRun bool
		expectedErr  string
	}{
		{
			name: "no dry run flags set",
		},
		{
			name:         "both dry run flags set",
			serverDryRun: true,
			dryRun:       true,
			expectedErr:  ErrIllegalDryRunFlagCombination.Error(),
		},
		{
			name:         "server dry run flag set",
			serverDryRun: true,
		},
		{
			name:   "dry run flag set",
			dryRun: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			o := NewApplyOptions(genericclioptions.NewTestIOStreamsDiscard())

			o.DryRun = test.dryRun
			o.ServerDryRun = test.serverDryRun

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
