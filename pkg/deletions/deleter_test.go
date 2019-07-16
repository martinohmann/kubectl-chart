package deletions

import (
	"strings"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/martinohmann/kubectl-chart/pkg/wait"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/cli-runtime/pkg/resource"
	dynamicfakeclient "k8s.io/client-go/dynamic/fake"
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

func TestDeleter_Delete(t *testing.T) {
	scheme := runtime.NewScheme()

	tests := []struct {
		name       string
		infos      []*resource.Info
		dryRun     bool
		fakeClient func() *dynamicfakeclient.FakeDynamicClient

		expectedErr     string
		validateActions func(t *testing.T, actions []clienttesting.Action)
	}{
		{
			name: "deletes a single resource",
			infos: []*resource.Info{
				{
					Mapping: &meta.RESTMapping{
						Resource: schema.GroupVersionResource{Group: "group", Version: "version", Resource: "theresource"},
					},
					Name:      "name-foo",
					Namespace: "ns-foo",
				},
			},
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				fakeClient := dynamicfakeclient.NewSimpleDynamicClient(scheme)
				fakeClient.PrependReactor("delete", "theresource", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, newUnstructuredList(newUnstructured("group/version", "TheKind", "ns-foo", "name-foo")), nil
				})
				return fakeClient
			},

			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 1 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("delete", "theresource") || actions[0].(clienttesting.DeleteAction).GetName() != "name-foo" {
					t.Error(spew.Sdump(actions))
				}
			},
		},
		{
			name:   "retrieves a single resource in dry run mode",
			dryRun: true,
			infos: []*resource.Info{
				{
					Mapping: &meta.RESTMapping{
						Resource: schema.GroupVersionResource{Group: "group", Version: "version", Resource: "theresource"},
					},
					Name:      "name-foo",
					Namespace: "ns-foo",
				},
			},
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				fakeClient := dynamicfakeclient.NewSimpleDynamicClient(scheme)
				fakeClient.PrependReactor("get", "theresource", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, newUnstructuredList(newUnstructured("group/version", "TheKind", "ns-foo", "name-foo")), nil
				})
				return fakeClient
			},

			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 1 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("get", "theresource") || actions[0].(clienttesting.GetAction).GetName() != "name-foo" {
					t.Error(spew.Sdump(actions))
				}
			},
		},
		{
			name: "continues on NotFound errors",
			infos: []*resource.Info{
				{
					Mapping: &meta.RESTMapping{
						Resource: schema.GroupVersionResource{Group: "group", Version: "version", Resource: "theresource"},
					},
					Name:      "name-foo",
					Namespace: "ns-foo",
				},
				{
					Mapping: &meta.RESTMapping{
						Resource: schema.GroupVersionResource{Group: "group", Version: "version", Resource: "theresource"},
					},
					Name:      "name-bar",
					Namespace: "ns-bar",
				},
			},
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				fakeClient := dynamicfakeclient.NewSimpleDynamicClient(scheme)
				fakeClient.PrependReactor("delete", "theresource", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					deleteAction := action.(clienttesting.DeleteAction)

					if deleteAction.GetName() == "name-foo" {
						return true, newUnstructuredList(newUnstructured("group/version", "TheKind", "ns-foo", "name-foo")), apierrors.NewNotFound(schema.GroupResource{Group: "group", Resource: "theresource"}, "name-foo")
					}

					return true, newUnstructuredList(newUnstructured("group/version", "TheKind", "ns-bar", "name-bar")), nil
				})
				return fakeClient
			},

			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("delete", "theresource") || actions[0].(clienttesting.DeleteAction).GetName() != "name-foo" {
					t.Error(spew.Sdump(actions))
				}

				if !actions[1].Matches("delete", "theresource") || actions[1].(clienttesting.DeleteAction).GetName() != "name-bar" {
					t.Error(spew.Sdump(actions))
				}
			},
		},
		{
			name: "aborts deletion for errors different from NotFoundError",
			infos: []*resource.Info{
				{
					Mapping: &meta.RESTMapping{
						Resource: schema.GroupVersionResource{Group: "group", Version: "version", Resource: "theresource"},
					},
					Name:      "name-foo",
					Namespace: "ns-foo",
				},
				{
					Mapping: &meta.RESTMapping{
						Resource: schema.GroupVersionResource{Group: "group", Version: "version", Resource: "theresource"},
					},
					Name:      "name-bar",
					Namespace: "ns-bar",
				},
			},
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				fakeClient := dynamicfakeclient.NewSimpleDynamicClient(scheme)
				fakeClient.PrependReactor("delete", "theresource", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					deleteAction := action.(clienttesting.DeleteAction)

					if deleteAction.GetName() == "name-foo" {
						return true, newUnstructuredList(newUnstructured("group/version", "TheKind", "ns-foo", "name-foo")), apierrors.NewUnauthorized("unauthorized")
					}

					return true, newUnstructuredList(newUnstructured("group/version", "TheKind", "ns-bar", "name-bar")), nil
				})
				return fakeClient
			},
			expectedErr: "unauthorized",
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 1 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("delete", "theresource") || actions[0].(clienttesting.DeleteAction).GetName() != "name-foo" {
					t.Error(spew.Sdump(actions))
				}
			},
		},
		{
			name:   "aborts dry run for errors different from NotFoundError",
			dryRun: true,
			infos: []*resource.Info{
				{
					Mapping: &meta.RESTMapping{
						Resource: schema.GroupVersionResource{Group: "group", Version: "version", Resource: "theresource"},
					},
					Name:      "name-foo",
					Namespace: "ns-foo",
				},
				{
					Mapping: &meta.RESTMapping{
						Resource: schema.GroupVersionResource{Group: "group", Version: "version", Resource: "theresource"},
					},
					Name:      "name-bar",
					Namespace: "ns-bar",
				},
			},
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				fakeClient := dynamicfakeclient.NewSimpleDynamicClient(scheme)
				fakeClient.PrependReactor("get", "theresource", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					getAction := action.(clienttesting.GetAction)

					if getAction.GetName() == "name-foo" {
						return true, newUnstructuredList(newUnstructured("group/version", "TheKind", "ns-foo", "name-foo")), apierrors.NewUnauthorized("unauthorized")
					}

					return true, newUnstructuredList(newUnstructured("group/version", "TheKind", "ns-bar", "name-bar")), nil
				})
				return fakeClient
			},
			expectedErr: "unauthorized",
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 1 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("get", "theresource") || actions[0].(clienttesting.GetAction).GetName() != "name-foo" {
					t.Error(spew.Sdump(actions))
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeClient := test.fakeClient()

			d := &deleter{
				IOStreams:     genericclioptions.NewTestIOStreamsDiscard(),
				DynamicClient: fakeClient,
				Printer:       printers.NewDiscardingPrinter(),
				Waiter:        wait.NewFakeWaiter(),
				DryRun:        test.dryRun,
			}

			err := d.Delete(resource.InfoListVisitor(test.infos))
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
		})
	}
}
