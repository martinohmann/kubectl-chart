package resources

import (
	"errors"
	"strings"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta/testrestmapper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
	fakediscovery "k8s.io/client-go/discovery/fake"
	dynamicfakeclient "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	clienttesting "k8s.io/client-go/testing"
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

func TestFinder_FindByLabelSelector(t *testing.T) {
	tests := []struct {
		name          string
		fakeClient    func() *dynamicfakeclient.FakeDynamicClient
		fakeDiscovery func() *fakediscovery.FakeDiscovery
		verbs         metav1.Verbs
		selector      string

		expectedErr              string
		validateDiscoveryActions func(t *testing.T, actions []clienttesting.Action)
		validateDynamicActions   func(t *testing.T, actions []clienttesting.Action)
		validateInfos            func(t *testing.T, infos []*resource.Info)
	}{
		{
			name:     "find resources that support any verbs",
			selector: "foo=bar",
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				fakeClient := dynamicfakeclient.NewSimpleDynamicClient(scheme.Scheme)
				fakeClient.PrependReactor("list", "pods", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, newUnstructuredList(newUnstructuredWithLabels("v1", "Pod", "ns-foo", "name-foo", map[string]interface{}{"foo": "bar"})), nil
				})
				fakeClient.PrependReactor("list", "daemonsets", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, newUnstructuredList(newUnstructuredWithLabels("apps/v1", "DaemonSet", "ns-foo", "name-foo", map[string]interface{}{"foo": "bar"})), nil
				})
				return fakeClient
			},
			fakeDiscovery: func() *fakediscovery.FakeDiscovery {
				fakeDiscovery := &fakediscovery.FakeDiscovery{Fake: &clienttesting.Fake{}}
				fakeDiscovery.Resources = []*metav1.APIResourceList{
					{
						GroupVersion: corev1.SchemeGroupVersion.String(),
						APIResources: []metav1.APIResource{
							{Name: "nodes", Namespaced: false, Kind: "Node"},
							{Name: "pods", Namespaced: true, Kind: "Pod", Verbs: metav1.Verbs{"sleep"}},
							{Name: "replicationcontrollers", Namespaced: true, Kind: "ReplicationController"},
						},
					},
					{
						GroupVersion: appsv1.SchemeGroupVersion.String(),
						APIResources: []metav1.APIResource{
							{Name: "daemonsets", Namespaced: true, Kind: "DaemonSet", Verbs: metav1.Verbs{"get"}},
							{Name: "statefulsets", Namespaced: true, Kind: "StatefulSet"},
						},
					},
				}

				return fakeDiscovery
			},
			validateDiscoveryActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}
			},
			validateDynamicActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}

				if !actions[0].Matches("list", "pods") || actions[0].(clienttesting.ListAction).GetListRestrictions().Labels.String() != "foo=bar" {
					t.Error(spew.Sdump(actions))
				}

				if !actions[1].Matches("list", "daemonsets") || actions[1].(clienttesting.ListAction).GetListRestrictions().Labels.String() != "foo=bar" {
					t.Error(spew.Sdump(actions))
				}
			},
			validateInfos: func(t *testing.T, infos []*resource.Info) {
				require.Len(t, infos, 2)

				assert.Equal(t, "Pod", infos[0].Object.GetObjectKind().GroupVersionKind().Kind)
				assert.Equal(t, "DaemonSet", infos[1].Object.GetObjectKind().GroupVersionKind().Kind)
			},
		},
		{
			name:     "find resources that support all required verbs",
			selector: "foo=bar",
			verbs:    metav1.Verbs{"list", "delete", "update"},
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				fakeClient := dynamicfakeclient.NewSimpleDynamicClient(scheme.Scheme)
				fakeClient.PrependReactor("list", "daemonsets", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, newUnstructuredList(newUnstructuredWithLabels("apps/v1", "DaemonSet", "ns-foo", "name-foo", map[string]interface{}{"foo": "bar"})), nil
				})
				return fakeClient
			},
			fakeDiscovery: func() *fakediscovery.FakeDiscovery {
				fakeDiscovery := &fakediscovery.FakeDiscovery{Fake: &clienttesting.Fake{}}
				fakeDiscovery.Resources = []*metav1.APIResourceList{
					{
						GroupVersion: corev1.SchemeGroupVersion.String(),
						APIResources: []metav1.APIResource{
							{Name: "nodes", Namespaced: false, Kind: "Node"},
							{Name: "pods", Namespaced: true, Kind: "Pod", Verbs: metav1.Verbs{"sleep"}},
							{Name: "replicationcontrollers", Namespaced: true, Kind: "ReplicationController"},
						},
					},
					{
						GroupVersion: appsv1.SchemeGroupVersion.String(),
						APIResources: []metav1.APIResource{
							{Name: "daemonsets", Namespaced: true, Kind: "DaemonSet", Verbs: metav1.Verbs{"list", "delete", "update"}},
							{Name: "statefulsets", Namespaced: true, Kind: "StatefulSet"},
						},
					},
				}

				return fakeDiscovery
			},
			validateDiscoveryActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}
			},
			validateDynamicActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 1 {
					t.Fatal(spew.Sdump(actions))
				}

				if !actions[0].Matches("list", "daemonsets") || actions[0].(clienttesting.ListAction).GetListRestrictions().Labels.String() != "foo=bar" {
					t.Error(spew.Sdump(actions))
				}
			},
			validateInfos: func(t *testing.T, infos []*resource.Info) {
				require.Len(t, infos, 1)

				assert.Equal(t, "DaemonSet", infos[0].Object.GetObjectKind().GroupVersionKind().Kind)
			},
		},
		{
			name:     "does not find duplicate GroupKinds twice",
			selector: "foo=bar",
			verbs:    metav1.Verbs{"list", "delete", "update"},
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				fakeClient := dynamicfakeclient.NewSimpleDynamicClient(scheme.Scheme)
				fakeClient.PrependReactor("list", "daemonsets", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, newUnstructuredList(newUnstructuredWithLabels("apps/v1", "DaemonSet", "ns-foo", "name-foo", map[string]interface{}{"foo": "bar"})), nil
				})
				return fakeClient
			},
			fakeDiscovery: func() *fakediscovery.FakeDiscovery {
				fakeDiscovery := &fakediscovery.FakeDiscovery{Fake: &clienttesting.Fake{}}
				fakeDiscovery.Resources = []*metav1.APIResourceList{
					{
						GroupVersion: corev1.SchemeGroupVersion.String(),
						APIResources: []metav1.APIResource{
							{Name: "nodes", Namespaced: false, Kind: "Node"},
							{Name: "pods", Namespaced: true, Kind: "Pod", Verbs: metav1.Verbs{"list"}},
							{Name: "replicationcontrollers", Namespaced: true, Kind: "ReplicationController"},
						},
					},
					{
						GroupVersion: appsv1.SchemeGroupVersion.String(),
						APIResources: []metav1.APIResource{
							{Name: "daemonsets", Namespaced: true, Kind: "DaemonSet", Verbs: metav1.Verbs{"list", "delete", "update"}},
							{Name: "daemonsets", Namespaced: true, Kind: "DaemonSet", Verbs: metav1.Verbs{"list", "delete", "update"}},
							{Name: "statefulsets", Namespaced: true, Kind: "StatefulSet"},
						},
					},
				}

				return fakeDiscovery
			},
			validateDiscoveryActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}
			},
			validateDynamicActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 1 {
					t.Fatal(spew.Sdump(actions))
				}

				if !actions[0].Matches("list", "daemonsets") || actions[0].(clienttesting.ListAction).GetListRestrictions().Labels.String() != "foo=bar" {
					t.Error(spew.Sdump(actions))
				}
			},
			validateInfos: func(t *testing.T, infos []*resource.Info) {
				require.Len(t, infos, 1)

				assert.Equal(t, "DaemonSet", infos[0].Object.GetObjectKind().GroupVersionKind().Kind)
			},
		},
		{
			name:     "ignores NotFound errors",
			selector: "foo=bar",
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				fakeClient := dynamicfakeclient.NewSimpleDynamicClient(scheme.Scheme)
				fakeClient.PrependReactor("list", "pods", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, apierrors.NewNotFound(schema.GroupResource{Resource: "pods"}, "name-foo")
				})
				fakeClient.PrependReactor("list", "daemonsets", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, newUnstructuredList(newUnstructuredWithLabels("apps/v1", "DaemonSet", "ns-foo", "name-foo", map[string]interface{}{"foo": "bar"})), nil
				})
				return fakeClient
			},
			fakeDiscovery: func() *fakediscovery.FakeDiscovery {
				fakeDiscovery := &fakediscovery.FakeDiscovery{Fake: &clienttesting.Fake{}}
				fakeDiscovery.Resources = []*metav1.APIResourceList{
					{
						GroupVersion: corev1.SchemeGroupVersion.String(),
						APIResources: []metav1.APIResource{
							{Name: "nodes", Namespaced: false, Kind: "Node"},
							{Name: "pods", Namespaced: true, Kind: "Pod", Verbs: metav1.Verbs{"get"}},
							{Name: "replicationcontrollers", Namespaced: true, Kind: "ReplicationController"},
						},
					},
					{
						GroupVersion: appsv1.SchemeGroupVersion.String(),
						APIResources: []metav1.APIResource{
							{Name: "daemonsets", Namespaced: true, Kind: "DaemonSet", Verbs: metav1.Verbs{"list"}},
							{Name: "statefulsets", Namespaced: true, Kind: "StatefulSet"},
						},
					},
				}

				return fakeDiscovery
			},
			validateDiscoveryActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}
			},
			validateDynamicActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}

				if !actions[0].Matches("list", "pods") || actions[0].(clienttesting.ListAction).GetListRestrictions().Labels.String() != "foo=bar" {
					t.Error(spew.Sdump(actions))
				}

				if !actions[1].Matches("list", "daemonsets") || actions[1].(clienttesting.ListAction).GetListRestrictions().Labels.String() != "foo=bar" {
					t.Error(spew.Sdump(actions))
				}
			},
			validateInfos: func(t *testing.T, infos []*resource.Info) {
				require.Len(t, infos, 1)

				assert.Equal(t, "DaemonSet", infos[0].Object.GetObjectKind().GroupVersionKind().Kind)
			},
		},
		{
			name:     "ignores Forbidden errors",
			selector: "foo=bar",
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				fakeClient := dynamicfakeclient.NewSimpleDynamicClient(scheme.Scheme)
				fakeClient.PrependReactor("list", "pods", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "pods"}, "name-foo", errors.New("not allowed"))
				})
				fakeClient.PrependReactor("list", "daemonsets", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, newUnstructuredList(newUnstructuredWithLabels("apps/v1", "DaemonSet", "ns-foo", "name-foo", map[string]interface{}{"foo": "bar"})), nil
				})
				return fakeClient
			},
			fakeDiscovery: func() *fakediscovery.FakeDiscovery {
				fakeDiscovery := &fakediscovery.FakeDiscovery{Fake: &clienttesting.Fake{}}
				fakeDiscovery.Resources = []*metav1.APIResourceList{
					{
						GroupVersion: corev1.SchemeGroupVersion.String(),
						APIResources: []metav1.APIResource{
							{Name: "nodes", Namespaced: false, Kind: "Node"},
							{Name: "pods", Namespaced: true, Kind: "Pod", Verbs: metav1.Verbs{"get"}},
							{Name: "replicationcontrollers", Namespaced: true, Kind: "ReplicationController"},
						},
					},
					{
						GroupVersion: appsv1.SchemeGroupVersion.String(),
						APIResources: []metav1.APIResource{
							{Name: "daemonsets", Namespaced: true, Kind: "DaemonSet", Verbs: metav1.Verbs{"list"}},
							{Name: "statefulsets", Namespaced: true, Kind: "StatefulSet"},
						},
					},
				}

				return fakeDiscovery
			},
			validateDiscoveryActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}
			},
			validateDynamicActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}

				if !actions[0].Matches("list", "pods") || actions[0].(clienttesting.ListAction).GetListRestrictions().Labels.String() != "foo=bar" {
					t.Error(spew.Sdump(actions))
				}

				if !actions[1].Matches("list", "daemonsets") || actions[1].(clienttesting.ListAction).GetListRestrictions().Labels.String() != "foo=bar" {
					t.Error(spew.Sdump(actions))
				}
			},
			validateInfos: func(t *testing.T, infos []*resource.Info) {
				require.Len(t, infos, 1)

				assert.Equal(t, "DaemonSet", infos[0].Object.GetObjectKind().GroupVersionKind().Kind)
			},
		},
		{
			name:        "does stop on errors unhandled API errors",
			expectedErr: `bad request`,
			selector:    "foo=bar",
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				fakeClient := dynamicfakeclient.NewSimpleDynamicClient(scheme.Scheme)
				fakeClient.PrependReactor("list", "pods", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, apierrors.NewBadRequest("bad request")
				})
				return fakeClient
			},
			fakeDiscovery: func() *fakediscovery.FakeDiscovery {
				fakeDiscovery := &fakediscovery.FakeDiscovery{Fake: &clienttesting.Fake{}}
				fakeDiscovery.Resources = []*metav1.APIResourceList{
					{
						GroupVersion: corev1.SchemeGroupVersion.String(),
						APIResources: []metav1.APIResource{
							{Name: "nodes", Namespaced: false, Kind: "Node"},
							{Name: "pods", Namespaced: true, Kind: "Pod", Verbs: metav1.Verbs{"get"}},
							{Name: "replicationcontrollers", Namespaced: true, Kind: "ReplicationController"},
						},
					},
					{
						GroupVersion: appsv1.SchemeGroupVersion.String(),
						APIResources: []metav1.APIResource{
							{Name: "daemonsets", Namespaced: true, Kind: "DaemonSet", Verbs: metav1.Verbs{"list"}},
							{Name: "statefulsets", Namespaced: true, Kind: "StatefulSet"},
						},
					},
				}

				return fakeDiscovery
			},
			validateDiscoveryActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}
			},
			validateDynamicActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 1 {
					t.Fatal(spew.Sdump(actions))
				}

				if !actions[0].Matches("list", "pods") || actions[0].(clienttesting.ListAction).GetListRestrictions().Labels.String() != "foo=bar" {
					t.Error(spew.Sdump(actions))
				}
			},
		},
		{
			name:        "does stop on discovery errors",
			expectedErr: `unexpected GroupVersion string: invalid/group/version`,
			selector:    "foo=bar",
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				return dynamicfakeclient.NewSimpleDynamicClient(scheme.Scheme)
			},
			fakeDiscovery: func() *fakediscovery.FakeDiscovery {
				fakeDiscovery := &fakediscovery.FakeDiscovery{Fake: &clienttesting.Fake{}}
				fakeDiscovery.Resources = []*metav1.APIResourceList{
					{
						GroupVersion: "invalid/group/version",
					},
				}

				return fakeDiscovery
			},
			validateDiscoveryActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 1 {
					t.Fatal(spew.Sdump(actions))
				}
			},
			validateDynamicActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 0 {
					t.Fatal(spew.Sdump(actions))
				}
			},
		},
		{
			name:        "does stop on mapping errors",
			expectedErr: `no matches for kind "FooBarBaz" in group "apps"`,
			selector:    "foo=bar",
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				return dynamicfakeclient.NewSimpleDynamicClient(scheme.Scheme)
			},
			fakeDiscovery: func() *fakediscovery.FakeDiscovery {
				fakeDiscovery := &fakediscovery.FakeDiscovery{Fake: &clienttesting.Fake{}}
				fakeDiscovery.Resources = []*metav1.APIResourceList{
					{
						GroupVersion: appsv1.SchemeGroupVersion.String(),
						APIResources: []metav1.APIResource{
							{Name: "daemonsets", Namespaced: true, Kind: "DaemonSet", Verbs: metav1.Verbs{"list"}},
							{Name: "foobarbaz", Namespaced: true, Kind: "FooBarBaz", Verbs: metav1.Verbs{"list"}},
							{Name: "statefulsets", Namespaced: true, Kind: "StatefulSet"},
						},
					},
				}

				return fakeDiscovery
			},
			validateDiscoveryActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}
			},
			validateDynamicActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 0 {
					t.Fatal(spew.Sdump(actions))
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fakeClient := tc.fakeClient()
			fakeDiscovery := tc.fakeDiscovery()
			testMapper := testrestmapper.TestOnlyStaticRESTMapper(scheme.Scheme)

			f := &Finder{
				DynamicClient:    fakeClient,
				SupportedVerbs:   tc.verbs,
				MappingDiscovery: NewMappingDiscovery(fakeDiscovery, testMapper),
			}

			infos, err := f.FindByLabelSelector(tc.selector)
			switch {
			case err == nil && len(tc.expectedErr) == 0:
			case err != nil && len(tc.expectedErr) == 0:
				t.Fatal(err)
			case err == nil && len(tc.expectedErr) != 0:
				t.Fatalf("missing: %q", tc.expectedErr)
			case err != nil && len(tc.expectedErr) != 0:
				if !strings.Contains(err.Error(), tc.expectedErr) {
					t.Fatalf("expected %q, got %q", tc.expectedErr, err.Error())
				}
			}

			if tc.validateDiscoveryActions != nil {
				tc.validateDiscoveryActions(t, fakeDiscovery.Actions())
			}

			if tc.validateDynamicActions != nil {
				tc.validateDynamicActions(t, fakeClient.Actions())
			}

			if tc.validateInfos != nil {
				tc.validateInfos(t, infos)
			}
		})
	}
}
