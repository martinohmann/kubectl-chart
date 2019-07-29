package statefulset

import (
	"strings"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/martinohmann/kubectl-chart/pkg/deletions"
	"github.com/martinohmann/kubectl-chart/pkg/meta"
	"k8s.io/apimachinery/pkg/api/meta/testrestmapper"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfakeclient "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	clienttesting "k8s.io/client-go/testing"
)

func newUnstructuredList(items ...*unstructured.Unstructured) *unstructured.UnstructuredList {
	list := &unstructured.UnstructuredList{}
	for i := range items {
		list.Items = append(list.Items, *items[i])
	}
	return list
}

func newUnstructured(apiVersion, kind, namespace, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]interface{}{
				"namespace": namespace,
				"name":      name,
				"uid":       "some-UID-value",
			},
		},
	}
}

func TestPersistentVolumeClaimPruner_PruneClaims(t *testing.T) {
	tests := []struct {
		name       string
		objs       []runtime.Object
		fakeClient func() *dynamicfakeclient.FakeDynamicClient

		expectedErr     string
		validateActions func(t *testing.T, actions []clienttesting.Action)
		validateDeleter func(t *testing.T, deleter *deletions.FakeDeleter)
	}{
		{
			name: "no StatefulSet",
			objs: []runtime.Object{
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]interface{}{
							"name": "bar",
						},
					},
				},
			},
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				return dynamicfakeclient.NewSimpleDynamicClient(scheme.Scheme)
			},
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 0 {
					t.Fatal(spew.Sdump(actions))
				}
			},
			validateDeleter: func(t *testing.T, deleter *deletions.FakeDeleter) {
				if deleter.Called != 0 {
					t.Fatal(spew.Sdump(deleter))
				}
			},
		},
		{
			name: "StatefulSet without deletion policy",
			objs: []runtime.Object{
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "StatefulSet",
						"metadata": map[string]interface{}{
							"name": "baz",
						},
					},
				},
			},
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				return dynamicfakeclient.NewSimpleDynamicClient(scheme.Scheme)
			},
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 0 {
					t.Fatal(spew.Sdump(actions))
				}
			},
			validateDeleter: func(t *testing.T, deleter *deletions.FakeDeleter) {
				if deleter.Called != 0 {
					t.Fatal(spew.Sdump(deleter))
				}
			},
		},
		{
			name: "deletes only PVCs for StatefulSets with correct deletion policy",
			objs: []runtime.Object{
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "StatefulSet",
						"metadata": map[string]interface{}{
							"annotations": map[string]interface{}{
								meta.AnnotationDeletionPolicy: meta.DeletionPolicyDeletePVCs.String(),
							},
							"name": "foo",
						},
					},
				},
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]interface{}{
							"name": "bar",
						},
					},
				},
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "StatefulSet",
						"metadata": map[string]interface{}{
							"name": "baz",
						},
					},
				},
			},
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				fakeClient := dynamicfakeclient.NewSimpleDynamicClient(scheme.Scheme)
				fakeClient.PrependReactor("list", "persistentvolumeclaims", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, newUnstructuredList(newUnstructured("v1", "PersistentVolumeClaim", "ns-foo", "name-foo")), nil
				})

				return fakeClient
			},
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 1 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("list", "persistentvolumeclaims") || actions[0].(clienttesting.ListAction).GetListRestrictions().Labels.String() != "kubectl-chart/owned-by-statefulset=foo" {
					t.Error(spew.Sdump(actions))
				}
			},
			validateDeleter: func(t *testing.T, deleter *deletions.FakeDeleter) {
				if deleter.Called != 1 {
					t.Fatal(spew.Sdump(deleter))
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeClient := test.fakeClient()
			deleter := deletions.NewFakeDeleter()

			pruner := NewPersistentVolumeClaimPruner(
				fakeClient,
				deleter,
				testrestmapper.TestOnlyStaticRESTMapper(scheme.Scheme),
			)

			err := pruner.PruneClaims(test.objs)
			switch {
			case err == nil && len(test.expectedErr) == 0:
			case err != nil && len(test.expectedErr) == 0:
				t.Fatal(err)
			case err == nil && len(test.expectedErr) != 0:
				t.Fatalf("missing: %q", test.expectedErr)
			case err != nil && len(test.expectedErr) != 0:
				if !strings.Contains(err.Error(), test.expectedErr) {
					t.Fatalf("expected %q, got %q", test.expectedErr, err.Error())
				}
			}

			test.validateActions(t, fakeClient.Actions())

			if test.validateDeleter != nil {
				test.validateDeleter(t, deleter)
			}
		})
	}
}
