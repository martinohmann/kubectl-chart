package wait

// This is the test suite the `kubectl wait` command uses with adjustments to
// added features to ensure that the waiting behaviour is similar.

import (
	"errors"
	"io/ioutil"
	"strings"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
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

func newUnstructuredWithUID(apiVersion, kind, namespace, name, uid string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]interface{}{
				"namespace": namespace,
				"name":      name,
				"uid":       uid,
			},
		},
	}
}

func newUnstructured(apiVersion, kind, namespace, name string) *unstructured.Unstructured {
	return newUnstructuredWithUID(apiVersion, kind, namespace, name, "some-UID-value")
}

func newUnstructuredStatus(status *metav1.Status) runtime.Unstructured {
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(status)
	if err != nil {
		panic(err)
	}
	return &unstructured.Unstructured{
		Object: obj,
	}
}

func addCondition(in *unstructured.Unstructured, name, status string) *unstructured.Unstructured {
	conditions, _, _ := unstructured.NestedSlice(in.Object, "status", "conditions")
	conditions = append(conditions, map[string]interface{}{
		"type":   name,
		"status": status,
	})
	unstructured.SetNestedSlice(in.Object, conditions, "status", "conditions")
	return in
}

func TestWaitForDeletion(t *testing.T) {
	scheme := runtime.NewScheme()

	tests := []struct {
		name       string
		infos      []*resource.Info
		fakeClient func() *dynamicfakeclient.FakeDynamicClient
		timeout    time.Duration
		uidMap     UIDMap

		expectedErr     string
		validateActions func(t *testing.T, actions []clienttesting.Action)
	}{
		{
			name: "missing on get",
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
				return dynamicfakeclient.NewSimpleDynamicClient(scheme)
			},
			timeout: 10 * time.Second,

			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 1 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("list", "theresource") || actions[0].(clienttesting.ListAction).GetListRestrictions().Fields.String() != "metadata.name=name-foo" {
					t.Error(spew.Sdump(actions))
				}
			},
		},
		{
			name: "uid conflict on get",
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
				fakeClient.PrependReactor("list", "theresource", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, newUnstructuredList(newUnstructured("group/version", "TheKind", "ns-foo", "name-foo")), nil
				})
				count := 0
				fakeClient.PrependWatchReactor("theresource", func(action clienttesting.Action) (handled bool, ret watch.Interface, err error) {
					if count == 0 {
						count++
						fakeWatch := watch.NewRaceFreeFake()
						go func() {
							time.Sleep(100 * time.Millisecond)
							fakeWatch.Stop()
						}()
						return true, fakeWatch, nil
					}
					fakeWatch := watch.NewRaceFreeFake()
					return true, fakeWatch, nil
				})
				return fakeClient
			},
			timeout: 10 * time.Second,
			uidMap: UIDMap{
				ResourceLocation{Namespace: "ns-foo", Name: "name-foo"}:                                                                               types.UID("some-UID-value"),
				ResourceLocation{GroupResource: schema.GroupResource{Group: "group", Resource: "theresource"}, Namespace: "ns-foo", Name: "name-foo"}: types.UID("some-nonmatching-UID-value"),
			},

			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 1 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("list", "theresource") || actions[0].(clienttesting.ListAction).GetListRestrictions().Fields.String() != "metadata.name=name-foo" {
					t.Error(spew.Sdump(actions))
				}
			},
		},
		{
			name: "times out",
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
				fakeClient.PrependReactor("list", "theresource", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, newUnstructuredList(newUnstructured("group/version", "TheKind", "ns-foo", "name-foo")), nil
				})
				return fakeClient
			},
			timeout: 1 * time.Second,

			expectedErr: "timed out waiting for the condition on theresource/name-foo",
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("list", "theresource") || actions[0].(clienttesting.ListAction).GetListRestrictions().Fields.String() != "metadata.name=name-foo" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[1].Matches("watch", "theresource") {
					t.Error(spew.Sdump(actions))
				}
			},
		},
		{
			name: "handles watch close out",
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
				fakeClient.PrependReactor("list", "theresource", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					unstructuredObj := newUnstructured("group/version", "TheKind", "ns-foo", "name-foo")
					unstructuredObj.SetResourceVersion("123")
					unstructuredList := newUnstructuredList(unstructuredObj)
					unstructuredList.SetResourceVersion("234")
					return true, unstructuredList, nil
				})
				count := 0
				fakeClient.PrependWatchReactor("theresource", func(action clienttesting.Action) (handled bool, ret watch.Interface, err error) {
					if count == 0 {
						count++
						fakeWatch := watch.NewRaceFreeFake()
						go func() {
							time.Sleep(100 * time.Millisecond)
							fakeWatch.Stop()
						}()
						return true, fakeWatch, nil
					}
					fakeWatch := watch.NewRaceFreeFake()
					return true, fakeWatch, nil
				})
				return fakeClient
			},
			timeout: 3 * time.Second,

			expectedErr: "timed out waiting for the condition on theresource/name-foo",
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 4 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("list", "theresource") || actions[0].(clienttesting.ListAction).GetListRestrictions().Fields.String() != "metadata.name=name-foo" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[1].Matches("watch", "theresource") || actions[1].(clienttesting.WatchAction).GetWatchRestrictions().ResourceVersion != "234" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[2].Matches("list", "theresource") || actions[2].(clienttesting.ListAction).GetListRestrictions().Fields.String() != "metadata.name=name-foo" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[3].Matches("watch", "theresource") || actions[3].(clienttesting.WatchAction).GetWatchRestrictions().ResourceVersion != "234" {
					t.Error(spew.Sdump(actions))
				}
			},
		},
		{
			name: "handles watch delete",
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
				fakeClient.PrependReactor("list", "theresource", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, newUnstructuredList(newUnstructured("group/version", "TheKind", "ns-foo", "name-foo")), nil
				})
				fakeClient.PrependWatchReactor("theresource", func(action clienttesting.Action) (handled bool, ret watch.Interface, err error) {
					fakeWatch := watch.NewRaceFreeFake()
					fakeWatch.Action(watch.Deleted, newUnstructured("group/version", "TheKind", "ns-foo", "name-foo"))
					return true, fakeWatch, nil
				})
				return fakeClient
			},
			timeout: 10 * time.Second,

			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("list", "theresource") || actions[0].(clienttesting.ListAction).GetListRestrictions().Fields.String() != "metadata.name=name-foo" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[1].Matches("watch", "theresource") {
					t.Error(spew.Sdump(actions))
				}
			},
		},
		{
			name: "handles watch delete multiple",
			infos: []*resource.Info{
				{
					Mapping: &meta.RESTMapping{
						Resource: schema.GroupVersionResource{Group: "group", Version: "version", Resource: "theresource-1"},
					},
					Name:      "name-foo-1",
					Namespace: "ns-foo",
				},
				{
					Mapping: &meta.RESTMapping{
						Resource: schema.GroupVersionResource{Group: "group", Version: "version", Resource: "theresource-2"},
					},
					Name:      "name-foo-2",
					Namespace: "ns-foo",
				},
			},
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				fakeClient := dynamicfakeclient.NewSimpleDynamicClient(scheme)
				fakeClient.PrependReactor("get", "theresource-1", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, newUnstructured("group/version", "TheKind", "ns-foo", "name-foo-1"), nil
				})
				fakeClient.PrependReactor("get", "theresource-2", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, newUnstructured("group/version", "TheKind", "ns-foo", "name-foo-2"), nil
				})
				fakeClient.PrependWatchReactor("theresource-1", func(action clienttesting.Action) (handled bool, ret watch.Interface, err error) {
					fakeWatch := watch.NewRaceFreeFake()
					fakeWatch.Action(watch.Deleted, newUnstructured("group/version", "TheKind", "ns-foo", "name-foo-1"))
					return true, fakeWatch, nil
				})
				fakeClient.PrependWatchReactor("theresource-2", func(action clienttesting.Action) (handled bool, ret watch.Interface, err error) {
					fakeWatch := watch.NewRaceFreeFake()
					fakeWatch.Action(watch.Deleted, newUnstructured("group/version", "TheKind", "ns-foo", "name-foo-2"))
					return true, fakeWatch, nil
				})
				return fakeClient
			},
			timeout: 10 * time.Second,

			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("list", "theresource-1") {
					t.Error(spew.Sdump(actions))
				}
				if !actions[1].Matches("list", "theresource-2") {
					t.Error(spew.Sdump(actions))
				}
			},
		},
		{
			name: "ignores watch error",
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
				fakeClient.PrependReactor("list", "theresource", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, newUnstructuredList(newUnstructured("group/version", "TheKind", "ns-foo", "name-foo")), nil
				})
				count := 0
				fakeClient.PrependWatchReactor("theresource", func(action clienttesting.Action) (handled bool, ret watch.Interface, err error) {
					fakeWatch := watch.NewRaceFreeFake()
					if count == 0 {
						fakeWatch.Error(newUnstructuredStatus(&metav1.Status{
							TypeMeta: metav1.TypeMeta{Kind: "Status", APIVersion: "v1"},
							Status:   "Failure",
							Code:     500,
							Message:  "Bad",
						}))
						fakeWatch.Stop()
					} else {
						fakeWatch.Action(watch.Deleted, newUnstructured("group/version", "TheKind", "ns-foo", "name-foo"))
					}
					count++
					return true, fakeWatch, nil
				})
				return fakeClient
			},
			timeout: 10 * time.Second,

			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 4 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("list", "theresource") || actions[0].(clienttesting.ListAction).GetListRestrictions().Fields.String() != "metadata.name=name-foo" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[1].Matches("watch", "theresource") {
					t.Error(spew.Sdump(actions))
				}
				if !actions[2].Matches("list", "theresource") || actions[2].(clienttesting.ListAction).GetListRestrictions().Fields.String() != "metadata.name=name-foo" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[3].Matches("watch", "theresource") {
					t.Error(spew.Sdump(actions))
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeClient := test.fakeClient()

			w := NewWaiter(genericclioptions.NewTestIOStreamsDiscard(), printers.NewDiscardingPrinter())

			req := &Request{
				Options: &Options{
					Timeout: test.timeout,
				},
				Visitor:     resource.InfoListVisitor(test.infos),
				ConditionFn: NewDeletedConditionFunc(fakeClient, ioutil.Discard, test.uidMap),
			}

			err := w.Wait(req)
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

func TestWaitForCompletion(t *testing.T) {
	scheme := runtime.NewScheme()

	tests := []struct {
		name       string
		infos      []*resource.Info
		fakeClient func() *dynamicfakeclient.FakeDynamicClient
		timeout    time.Duration

		expectedErr     string
		validateActions func(t *testing.T, actions []clienttesting.Action)
	}{
		{
			name: "present on get",
			infos: []*resource.Info{
				{
					Mapping: &meta.RESTMapping{
						Resource:         schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "theresource"},
						GroupVersionKind: jobGVK,
					},
					Name:      "name-foo",
					Namespace: "ns-foo",
				},
			},
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				fakeClient := dynamicfakeclient.NewSimpleDynamicClient(scheme)
				fakeClient.PrependReactor("list", "theresource", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, newUnstructuredList(addCondition(
						newUnstructured("group/version", "TheKind", "ns-foo", "name-foo"),
						"complete", "true",
					)), nil
				})
				return fakeClient
			},
			timeout: 10 * time.Second,

			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 1 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("list", "theresource") || actions[0].(clienttesting.ListAction).GetListRestrictions().Fields.String() != "metadata.name=name-foo" {
					t.Error(spew.Sdump(actions))
				}
			},
		},
		{
			name: "handles job failure",
			infos: []*resource.Info{
				{
					Mapping: &meta.RESTMapping{
						Resource:         schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "theresource"},
						GroupVersionKind: jobGVK,
					},
					Name:      "name-foo",
					Namespace: "ns-foo",
				},
			},
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				fakeClient := dynamicfakeclient.NewSimpleDynamicClient(scheme)
				fakeClient.PrependReactor("list", "theresource", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, newUnstructuredList(addCondition(
						newUnstructured("group/version", "TheKind", "ns-foo", "name-foo"),
						"failed", "true",
					)), nil
				})
				return fakeClient
			},
			timeout: 10 * time.Second,
			expectedErr: StatusFailedError{
				Name:             "name-foo",
				GroupVersionKind: schema.GroupVersionKind{Group: "group", Version: "version", Kind: "TheKind"},
			}.Error(),

			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 1 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("list", "theresource") || actions[0].(clienttesting.ListAction).GetListRestrictions().Fields.String() != "metadata.name=name-foo" {
					t.Error(spew.Sdump(actions))
				}
			},
		},
		{
			name: "skips non-job resources",
			infos: []*resource.Info{
				{
					Mapping: &meta.RESTMapping{
						Resource: schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "theresource"},
					},
					Namespace: "ns-foo",
				},
			},
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				return dynamicfakeclient.NewSimpleDynamicClient(scheme)
			},
			timeout: 10 * time.Second,

			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 0 {
					t.Fatal(spew.Sdump(actions))
				}
			},
		},
		{
			name: "handles empty object name",
			infos: []*resource.Info{
				{
					Mapping: &meta.RESTMapping{
						Resource:         schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "theresource"},
						GroupVersionKind: jobGVK,
					},
					Namespace: "ns-foo",
				},
			},
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				return dynamicfakeclient.NewSimpleDynamicClient(scheme)
			},
			timeout:     10 * time.Second,
			expectedErr: "resource name must be provided",

			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 0 {
					t.Fatal(spew.Sdump(actions))
				}
			},
		},
		{
			name: "times out",
			infos: []*resource.Info{
				{
					Mapping: &meta.RESTMapping{
						Resource:         schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "theresource"},
						GroupVersionKind: jobGVK,
					},
					Name:      "name-foo",
					Namespace: "ns-foo",
				},
			},
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				fakeClient := dynamicfakeclient.NewSimpleDynamicClient(scheme)
				fakeClient.PrependReactor("list", "theresource", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, addCondition(
						newUnstructured("group/version", "TheKind", "ns-foo", "name-foo"),
						"some-other-condition", "status-value",
					), nil
				})
				return fakeClient
			},
			timeout: 1 * time.Second,

			expectedErr: "timed out waiting for the condition on theresource/name-foo",
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("list", "theresource") || actions[0].(clienttesting.ListAction).GetListRestrictions().Fields.String() != "metadata.name=name-foo" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[1].Matches("watch", "theresource") {
					t.Error(spew.Sdump(actions))
				}
			},
		},
		{
			name: "handles watch close out",
			infos: []*resource.Info{
				{
					Mapping: &meta.RESTMapping{
						Resource:         schema.GroupVersionResource{Group: "group", Version: "version", Resource: "theresource"},
						GroupVersionKind: jobGVK,
					},
					Name:      "name-foo",
					Namespace: "ns-foo",
				},
			},
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				fakeClient := dynamicfakeclient.NewSimpleDynamicClient(scheme)
				fakeClient.PrependReactor("list", "theresource", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					unstructuredObj := newUnstructured("group/version", "TheKind", "ns-foo", "name-foo")
					unstructuredObj.SetResourceVersion("123")
					unstructuredList := newUnstructuredList(unstructuredObj)
					unstructuredList.SetResourceVersion("234")
					return true, unstructuredList, nil
				})
				count := 0
				fakeClient.PrependWatchReactor("theresource", func(action clienttesting.Action) (handled bool, ret watch.Interface, err error) {
					if count == 0 {
						count++
						fakeWatch := watch.NewRaceFreeFake()
						go func() {
							time.Sleep(100 * time.Millisecond)
							fakeWatch.Stop()
						}()
						return true, fakeWatch, nil
					}
					fakeWatch := watch.NewRaceFreeFake()
					return true, fakeWatch, nil
				})
				return fakeClient
			},
			timeout: 3 * time.Second,

			expectedErr: "timed out waiting for the condition on theresource/name-foo",
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 4 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("list", "theresource") || actions[0].(clienttesting.ListAction).GetListRestrictions().Fields.String() != "metadata.name=name-foo" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[1].Matches("watch", "theresource") || actions[1].(clienttesting.WatchAction).GetWatchRestrictions().ResourceVersion != "234" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[2].Matches("list", "theresource") || actions[2].(clienttesting.ListAction).GetListRestrictions().Fields.String() != "metadata.name=name-foo" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[3].Matches("watch", "theresource") || actions[3].(clienttesting.WatchAction).GetWatchRestrictions().ResourceVersion != "234" {
					t.Error(spew.Sdump(actions))
				}
			},
		},
		{
			name: "handles watch condition change",
			infos: []*resource.Info{
				{
					Mapping: &meta.RESTMapping{
						Resource:         schema.GroupVersionResource{Group: "group", Version: "version", Resource: "theresource"},
						GroupVersionKind: jobGVK,
					},
					Name:      "name-foo",
					Namespace: "ns-foo",
				},
			},
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				fakeClient := dynamicfakeclient.NewSimpleDynamicClient(scheme)
				fakeClient.PrependReactor("list", "theresource", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, newUnstructuredList(newUnstructured("group/version", "TheKind", "ns-foo", "name-foo")), nil
				})
				fakeClient.PrependWatchReactor("theresource", func(action clienttesting.Action) (handled bool, ret watch.Interface, err error) {
					fakeWatch := watch.NewRaceFreeFake()
					fakeWatch.Action(watch.Modified, addCondition(
						newUnstructured("group/version", "TheKind", "ns-foo", "name-foo"),
						"complete", "true",
					))
					return true, fakeWatch, nil
				})
				return fakeClient
			},
			timeout: 10 * time.Second,

			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("list", "theresource") || actions[0].(clienttesting.ListAction).GetListRestrictions().Fields.String() != "metadata.name=name-foo" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[1].Matches("watch", "theresource") {
					t.Error(spew.Sdump(actions))
				}
			},
		},
		{
			name: "handles watch created",
			infos: []*resource.Info{
				{
					Mapping: &meta.RESTMapping{
						Resource:         schema.GroupVersionResource{Group: "group", Version: "version", Resource: "theresource"},
						GroupVersionKind: jobGVK,
					},
					Name:      "name-foo",
					Namespace: "ns-foo",
				},
			},
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				fakeClient := dynamicfakeclient.NewSimpleDynamicClient(scheme)
				fakeClient.PrependWatchReactor("theresource", func(action clienttesting.Action) (handled bool, ret watch.Interface, err error) {
					fakeWatch := watch.NewRaceFreeFake()
					fakeWatch.Action(watch.Added, addCondition(
						newUnstructured("group/version", "TheKind", "ns-foo", "name-foo"),
						"complete", "true",
					))
					return true, fakeWatch, nil
				})
				return fakeClient
			},
			timeout: 10 * time.Second,

			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("list", "theresource") || actions[0].(clienttesting.ListAction).GetListRestrictions().Fields.String() != "metadata.name=name-foo" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[1].Matches("watch", "theresource") {
					t.Error(spew.Sdump(actions))
				}
			},
		},
		{
			name: "ignores watch error",
			infos: []*resource.Info{
				{
					Mapping: &meta.RESTMapping{
						Resource:         schema.GroupVersionResource{Group: "group", Version: "version", Resource: "theresource"},
						GroupVersionKind: jobGVK,
					},
					Name:      "name-foo",
					Namespace: "ns-foo",
				},
			},
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				fakeClient := dynamicfakeclient.NewSimpleDynamicClient(scheme)
				fakeClient.PrependReactor("list", "theresource", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, newUnstructuredList(newUnstructured("group/version", "TheKind", "ns-foo", "name-foo")), nil
				})
				count := 0
				fakeClient.PrependWatchReactor("theresource", func(action clienttesting.Action) (handled bool, ret watch.Interface, err error) {
					fakeWatch := watch.NewRaceFreeFake()
					if count == 0 {
						fakeWatch.Error(newUnstructuredStatus(&metav1.Status{
							TypeMeta: metav1.TypeMeta{Kind: "Status", APIVersion: "v1"},
							Status:   "Failure",
							Code:     500,
							Message:  "Bad",
						}))
						fakeWatch.Stop()
					} else {
						fakeWatch.Action(watch.Modified, addCondition(
							newUnstructured("group/version", "TheKind", "ns-foo", "name-foo"),
							"complete", "true",
						))
					}
					count++
					return true, fakeWatch, nil
				})
				return fakeClient
			},
			timeout: 10 * time.Second,

			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 4 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("list", "theresource") || actions[0].(clienttesting.ListAction).GetListRestrictions().Fields.String() != "metadata.name=name-foo" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[1].Matches("watch", "theresource") {
					t.Error(spew.Sdump(actions))
				}
				if !actions[2].Matches("list", "theresource") || actions[2].(clienttesting.ListAction).GetListRestrictions().Fields.String() != "metadata.name=name-foo" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[3].Matches("watch", "theresource") {
					t.Error(spew.Sdump(actions))
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeClient := test.fakeClient()

			w := NewSilentWaiter(genericclioptions.NewTestIOStreamsDiscard())

			req := &Request{
				Options: &Options{
					Timeout: test.timeout,
				},
				Visitor:     resource.InfoListVisitor(test.infos),
				ConditionFn: NewCompletionConditionFunc(fakeClient, ioutil.Discard),
			}

			err := w.Wait(req)
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

func TestWait_Errors(t *testing.T) {
	tests := []struct {
		name        string
		conditionFn ConditionFunc
		options     Options
		expectedErr string
	}{
		{
			name: "forces error if condition was not satisfied and no error was returned",
			conditionFn: func(info *resource.Info, o Options) (runtime.Object, bool, error) {
				return newUnstructured("batch", "v1", "ns-foo", "name-foo"), false, nil
			},
			expectedErr: "&{map[apiVersion:batch kind:v1 metadata:map[name:name-foo namespace:ns-foo uid:some-UID-value]]} unsatisified for unknown reason",
		},
		{
			name: "does not ignore WaitTimeoutError if AllowFailure is not set",
			conditionFn: func(info *resource.Info, o Options) (runtime.Object, bool, error) {
				err := &WaitTimeoutError{
					Name:     "foo",
					Resource: "jobs",
					Err:      errors.New("foobar"),
				}
				return nil, false, err
			},
			expectedErr: "foobar on jobs/foo",
		},
		{
			name: "ignores WaitTimeoutError if AllowFailure is set",
			options: Options{
				AllowFailure: true,
			},
			conditionFn: func(info *resource.Info, o Options) (runtime.Object, bool, error) {
				err := &WaitTimeoutError{
					Name:     "foo",
					Resource: "jobs",
					Err:      errors.New("foobar"),
				}
				return nil, false, err
			},
		},
		{
			name: "does not ignore StatusFailedError if AllowFailure is not set",
			conditionFn: func(info *resource.Info, o Options) (runtime.Object, bool, error) {
				err := &StatusFailedError{
					Name: "foo",
					GroupVersionKind: schema.GroupVersionKind{
						Group:   "batch",
						Version: "v1",
						Kind:    "Job",
					},
				}
				return nil, false, err
			},
			expectedErr: `batch/v1, Kind=Job "foo" is in status failed`,
		},
		{
			name: "ignores StatusFailedError if AllowFailure is set",
			options: Options{
				AllowFailure: true,
			},
			conditionFn: func(info *resource.Info, o Options) (runtime.Object, bool, error) {
				err := &StatusFailedError{
					Name: "foo",
					GroupVersionKind: schema.GroupVersionKind{
						Group:   "batch",
						Version: "v1",
						Kind:    "Job",
					},
				}
				return nil, false, err
			},
		},
		{
			name: "ignores WaitSkippedError",
			conditionFn: func(info *resource.Info, o Options) (runtime.Object, bool, error) {
				err := &WaitSkippedError{
					Name: "foo",
					GroupVersionKind: schema.GroupVersionKind{
						Group:   "batch",
						Version: "v1",
						Kind:    "Job",
					},
				}
				return nil, false, err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			w := NewSilentWaiter(genericclioptions.NewTestIOStreamsDiscard())

			req := &Request{
				Options: &test.options,
				Visitor: resource.InfoListVisitor([]*resource.Info{
					{Object: newUnstructured("batch/v1", "Job", "ns-foo", "name-foo")},
				}),
				ConditionFn: test.conditionFn,
			}

			err := w.Wait(req)
			if test.expectedErr != "" {
				require.Error(t, err)
				assert.Equal(t, test.expectedErr, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRequest_OptionsFor(t *testing.T) {
	tests := []struct {
		name            string
		options         *Options
		resourceOptions ResourceOptions
		obj             runtime.Object
		validate        func(t *testing.T, opts Options)
	}{
		{
			name: "falls back to default options if no resource options are defined",
			obj:  newUnstructuredWithUID("batch/v1", "job", "ns-foo", "name-foo", ""),
			validate: func(t *testing.T, opts Options) {
				assert.Equal(t, DefaultOptions, opts)
			},
		},
		{
			name: "falls back to default options if no resource options are defined for UID",
			obj:  newUnstructuredWithUID("batch/v1", "job", "ns-foo", "name-foo", "the-uid"),
			resourceOptions: ResourceOptions{
				"other-uid": Options{
					Timeout:      10 * time.Second,
					AllowFailure: true,
				},
			},
			validate: func(t *testing.T, opts Options) {
				assert.Equal(t, DefaultOptions, opts)
			},
		},
		{
			name: "uses options for UID",
			obj:  newUnstructuredWithUID("batch/v1", "job", "ns-foo", "name-foo", "the-uid"),
			resourceOptions: ResourceOptions{
				"the-uid": Options{
					Timeout:      20 * time.Second,
					AllowFailure: true,
				},
			},
			validate: func(t *testing.T, opts Options) {
				assert.Equal(t, 20*time.Second, opts.Timeout)
				assert.True(t, opts.AllowFailure)
			},
		},
		{
			name: "does not fall back to default options if options are defined",
			obj:  newUnstructuredWithUID("batch/v1", "job", "ns-foo", "name-foo", "the-uid"),
			options: &Options{
				Timeout:      10 * time.Second,
				AllowFailure: true,
			},
			validate: func(t *testing.T, opts Options) {
				assert.Equal(t, 10*time.Second, opts.Timeout)
				assert.True(t, opts.AllowFailure)
			},
		},
		{
			name: "resource options have precedence",
			obj:  newUnstructuredWithUID("batch/v1", "job", "ns-foo", "name-foo", "the-uid"),
			resourceOptions: ResourceOptions{
				"the-uid": Options{
					Timeout:      5 * time.Minute,
					AllowFailure: true,
				},
			},
			options: &Options{
				Timeout: 10 * time.Second,
			},
			validate: func(t *testing.T, opts Options) {
				assert.Equal(t, 5*time.Minute, opts.Timeout)
				assert.True(t, opts.AllowFailure)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := &Request{
				Options:         test.options,
				ResourceOptions: test.resourceOptions,
			}

			opts := r.OptionsFor(&resource.Info{Object: test.obj})

			test.validate(t, opts)
		})
	}
}
