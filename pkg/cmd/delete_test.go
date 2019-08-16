package cmd

import (
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/martinohmann/kubectl-chart/pkg/resources"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	fakediscovery "k8s.io/client-go/discovery/fake"
	dynamicfakeclient "k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
)

func newUnstructuredList(items ...*unstructured.Unstructured) *unstructured.UnstructuredList {
	list := &unstructured.UnstructuredList{}
	for i := range items {
		item := *items[i]
		list.Items = append(list.Items, item)
	}
	return list
}

func newUnstructured(apiVersion, kind, namespace, name string) *unstructured.Unstructured {
	return newUnstructuredWithLabels(apiVersion, kind, namespace, name, nil)
}

func newUnstructuredWithLabels(apiVersion, kind, namespace, name string, labels map[string]interface{}) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]interface{}{
				"namespace": namespace,
				"name":      name,
				"labels":    labels,
			},
		},
	}
}

func TestDeleteCmd(t *testing.T) {
	cmdtesting.InitTestErrorHandler(t)

	f := cmdtesting.NewTestFactory().WithNamespace("test")
	f.ClientConfigVal = cmdtesting.DefaultClientConfig()
	defer f.Cleanup()

	fakeClient := dynamicfakeclient.NewSimpleDynamicClient(runtime.NewScheme())

	o := NewDeleteOptions(genericclioptions.NewTestIOStreamsDiscard())

	o.ChartFlags.ChartDir = "../chart/testdata/valid-charts/chart1"
	o.DynamicClientGetter.Client = fakeClient

	require.NoError(t, o.Complete(f))
	require.NoError(t, o.Run())

	actions := fakeClient.Actions()

	if len(actions) != 2 {
		t.Fatal(spew.Sdump(actions))
	}

	if !actions[0].Matches("delete", "services") || actions[0].(clienttesting.DeleteAction).GetName() != "chart1" {
		t.Error(spew.Sdump(actions))
	}

	if !actions[1].Matches("delete", "statefulsets") || actions[1].(clienttesting.DeleteAction).GetName() != "chart1" {
		t.Error(spew.Sdump(actions))
	}
}

func TestDeleteCmd_DryRun(t *testing.T) {
	cmdtesting.InitTestErrorHandler(t)

	f := cmdtesting.NewTestFactory().WithNamespace("test")
	f.ClientConfigVal = cmdtesting.DefaultClientConfig()
	defer f.Cleanup()

	streams, _, buf, _ := genericclioptions.NewTestIOStreams()

	fakeClient := dynamicfakeclient.NewSimpleDynamicClient(runtime.NewScheme())
	fakeClient.PrependReactor("get", "services", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, newUnstructuredWithLabels("v1", "Service", "test", "chart1", map[string]interface{}{"kubectl-chart/chart-name": "chart1"}), nil
	})
	fakeClient.PrependReactor("get", "statefulsets", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, newUnstructuredWithLabels("apps/v1", "StatefulSet", "test", "chart1", map[string]interface{}{"kubectl-chart/chart-name": "chart1"}), nil
	})

	o := NewDeleteOptions(streams)

	o.ChartFlags.ChartDir = "../chart/testdata/valid-charts/chart1"
	o.DryRun = true
	o.DynamicClientGetter.Client = fakeClient

	require.NoError(t, o.Complete(f))
	require.NoError(t, o.Run())

	actions := fakeClient.Actions()

	if len(actions) != 2 {
		t.Fatal(spew.Sdump(actions))
	}

	if !actions[0].Matches("get", "services") || actions[0].(clienttesting.GetAction).GetName() != "chart1" {
		t.Error(spew.Sdump(actions))
	}

	if !actions[1].Matches("get", "statefulsets") || actions[1].(clienttesting.GetAction).GetName() != "chart1" {
		t.Error(spew.Sdump(actions))
	}

	expected := `service/chart1 deleted (dry run)
statefulset.apps/chart1 deleted (dry run)
`

	assert.Equal(t, expected, buf.String())
}

func TestDeleteCmd_Prune(t *testing.T) {
	cmdtesting.InitTestErrorHandler(t)

	fakeClient := dynamicfakeclient.NewSimpleDynamicClient(runtime.NewScheme())
	fakeClient.PrependReactor("list", "pods", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, newUnstructuredList(newUnstructuredWithLabels("", "Pod", "ns-foo", "name-foo", map[string]interface{}{"kubectl-chart/chart-name": "chart1"})), nil
	})

	fakeDiscovery := &fakediscovery.FakeDiscovery{Fake: &clienttesting.Fake{}}
	fakeDiscovery.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: corev1.SchemeGroupVersion.String(),
			APIResources: []metav1.APIResource{
				{Name: "pods", Namespaced: true, Kind: "Pod", Verbs: resources.DefaultSupportedVerbs},
			},
		},
		{
			GroupVersion: appsv1.SchemeGroupVersion.String(),
			APIResources: []metav1.APIResource{
				{Name: "statefulsets", Namespaced: true, Kind: "StatefulSet", Verbs: resources.DefaultSupportedVerbs},
			},
		},
	}

	f := newTestFactoryWithFakeDiscovery(fakeDiscovery)
	f.ClientConfigVal = cmdtesting.DefaultClientConfig()
	defer f.Cleanup()

	o := NewDeleteOptions(genericclioptions.NewTestIOStreamsDiscard())

	o.ChartFlags.ChartDir = "../chart/testdata/valid-charts/chart1"
	o.Prune = true
	o.DynamicClientGetter.Client = fakeClient

	require.NoError(t, o.Complete(f))
	require.NoError(t, o.Run())

	actions := fakeClient.Actions()

	if len(actions) != 3 {
		t.Fatal(spew.Sdump(actions))
	}

	if !actions[0].Matches("list", "pods") || actions[0].(clienttesting.ListAction).GetListRestrictions().Labels.String() != "kubectl-chart/chart-name=chart1" {
		t.Error(spew.Sdump(actions))
	}

	if !actions[1].Matches("list", "statefulsets") || actions[0].(clienttesting.ListAction).GetListRestrictions().Labels.String() != "kubectl-chart/chart-name=chart1" {
		t.Error(spew.Sdump(actions))
	}

	if !actions[2].Matches("delete", "pods") || actions[2].(clienttesting.DeleteAction).GetName() != "name-foo" {
		t.Error(spew.Sdump(actions))
	}
}
