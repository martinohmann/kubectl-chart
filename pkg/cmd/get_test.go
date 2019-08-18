package cmd

import (
	"net/http"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
)

func pod(namespace, name string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func podList(pods ...*corev1.Pod) *corev1.PodList {
	l := &corev1.PodList{
		Items: make([]corev1.Pod, len(pods)),
	}

	for i, p := range pods {
		l.Items[i] = *p
	}

	return l
}

func TestGetOptionsCmd(t *testing.T) {
	cmdtesting.InitTestErrorHandler(t)

	f := newTestFactoryWithFakeDiscovery(nil)
	f.UnstructuredClient = &fake.RESTClient{
		NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			switch p, m := req.URL.Path, req.Method; {
			case m == "GET" && p == "/pods" && req.URL.RawQuery == "labelSelector=kubectl-chart%2Fchart-name&limit=500":
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     cmdtesting.DefaultHeader(),
					Body: cmdtesting.ObjBody(
						codec,
						podList(pod("kube-system", "foo")),
					),
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

	cmd := NewGetCmd(f, streams)

	// the cmd requires to have a parent, so we just set an empty one
	parent := &cobra.Command{}
	parent.AddCommand(cmd)

	parent.SetArgs([]string{"get", "pod"})

	require.NoError(t, parent.Execute())

	expected := `NAMESPACE     NAME   AGE
kube-system   foo    <unknown>
`
	assert.Equal(t, expected, buf.String())
}

func TestGetOptionsCmd_withChartNames(t *testing.T) {
	cmdtesting.InitTestErrorHandler(t)

	f := newTestFactoryWithFakeDiscovery(nil)
	f.UnstructuredClient = &fake.RESTClient{
		NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			switch p, m := req.URL.Path, req.Method; {
			case m == "GET" && p == "/pods" && req.URL.RawQuery == "labelSelector=kubectl-chart%2Fchart-name+in+%28chart1%2Cchart2%29&limit=500":
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     cmdtesting.DefaultHeader(),
					Body: cmdtesting.ObjBody(
						codec,
						podList(pod("default", "foo"), pod("kube-system", "bar")),
					),
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

	cmd := NewGetCmd(f, streams)

	// the cmd requires to have a parent, so we just set an empty one
	parent := &cobra.Command{}
	parent.AddCommand(cmd)

	parent.SetArgs([]string{"get", "pod", "-c", "chart1,chart2"})

	require.NoError(t, parent.Execute())

	expected := `NAMESPACE     NAME   AGE
default       foo    <unknown>
kube-system   bar    <unknown>
`
	assert.Equal(t, expected, buf.String())
}
