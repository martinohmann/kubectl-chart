package chart

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/martinohmann/kubectl-chart/pkg/deletions"
	"github.com/martinohmann/kubectl-chart/pkg/hook"
	"github.com/martinohmann/kubectl-chart/pkg/meta"
	"github.com/martinohmann/kubectl-chart/pkg/printers"
	"github.com/martinohmann/kubectl-chart/pkg/wait"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kmeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/meta/testrestmapper"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	dynamicfakeclient "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest/fake"
	"k8s.io/client-go/restmapper"
	clienttesting "k8s.io/client-go/testing"
)

func TestHookExecutor_ExecHooks(t *testing.T) {
	cases := []struct {
		name       string
		fakeClient func() *dynamicfakeclient.FakeDynamicClient
		hooks      hook.Map
		hookType   string
		dryRun     bool

		expectedErr          string
		validateActions      func(t *testing.T, actions []clienttesting.Action)
		validateDeletions    func(t *testing.T, deleter *deletions.FakeDeleter)
		validateWaitRequests func(t *testing.T, reqs []*wait.Request)
	}{
		{
			name: "execute one hook",
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				return dynamicfakeclient.NewSimpleDynamicClient(scheme.Scheme)
			},
			hookType: hook.TypePreApply,
			hooks: hook.Map{
				hook.TypePreApply: hook.List{
					hook.MustParse(&unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "batch/v1",
							"kind":       "Job",
							"metadata": map[string]interface{}{
								"name":      "somehook",
								"namespace": "bar",
								"annotations": map[string]interface{}{
									meta.AnnotationHookType: hook.TypePreApply,
								},
								"labels": map[string]interface{}{
									meta.LabelHookChartName: "foochart",
									meta.LabelHookType:      hook.TypePreApply,
								},
							},
							"spec": map[string]interface{}{
								"template": map[string]interface{}{
									"spec": map[string]interface{}{
										"restartPolicy": "Never",
									},
								},
							},
						},
					}),
				},
			},
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}

				if actions[0].GetVerb() != "list" {
					t.Error(spew.Sdump(actions))
				}

				if !actions[1].Matches("create", "jobs") {
					t.Error(spew.Sdump(actions))
				}

				obj := actions[1].(clienttesting.CreateAction).GetObject()

				metadata, err := kmeta.Accessor(obj)
				if err != nil {
					t.Fatal(err)
				}

				if metadata.GetName() != "somehook" {
					t.Fatalf("expected hook %q, got %q", "somehook", metadata.GetName())
				}
			},
			validateWaitRequests: func(t *testing.T, reqs []*wait.Request) {
				if len(reqs) != 1 {
					t.Fatal(spew.Sdump(reqs))
				}

				if len(reqs[0].Visitor.(resource.InfoListVisitor)) != 1 {
					t.Fatal(spew.Sdump(reqs[0].Visitor))
				}

				if len(reqs[0].ResourceOptions) != 0 {
					t.Fatal(spew.Sdump(reqs[0].ResourceOptions))
				}
			},
		},
		{
			name: "don't wait if hook has no-wait annotation",
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				return dynamicfakeclient.NewSimpleDynamicClient(scheme.Scheme)
			},
			hookType: hook.TypePreApply,
			hooks: hook.Map{
				hook.TypePreApply: hook.List{
					hook.MustParse(&unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "batch/v1",
							"kind":       "Job",
							"metadata": map[string]interface{}{
								"name":      "somehook",
								"namespace": "bar",
								"annotations": map[string]interface{}{
									meta.AnnotationHookType:   hook.TypePreApply,
									meta.AnnotationHookNoWait: "true",
								},
								"labels": map[string]interface{}{
									meta.LabelHookChartName: "foochart",
									meta.LabelHookType:      hook.TypePreApply,
								},
							},
							"spec": map[string]interface{}{
								"template": map[string]interface{}{
									"spec": map[string]interface{}{
										"restartPolicy": "Never",
									},
								},
							},
						},
					}),
				},
			},
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}

				if actions[0].GetVerb() != "list" {
					t.Error(spew.Sdump(actions))
				}

				if !actions[1].Matches("create", "jobs") {
					t.Error(spew.Sdump(actions))
				}

				obj := actions[1].(clienttesting.CreateAction).GetObject()

				metadata, err := kmeta.Accessor(obj)
				if err != nil {
					t.Fatal(err)
				}

				if metadata.GetName() != "somehook" {
					t.Fatalf("expected hook %q, got %q", "somehook", metadata.GetName())
				}
			},
			validateWaitRequests: func(t *testing.T, reqs []*wait.Request) {
				if len(reqs) != 0 {
					t.Fatal(spew.Sdump(reqs))
				}
			},
		},
		{
			name: "execute one hook with custom options",
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				return dynamicfakeclient.NewSimpleDynamicClient(scheme.Scheme)
			},
			hookType: hook.TypePreApply,
			hooks: hook.Map{
				hook.TypePreApply: hook.List{
					hook.MustParse(&unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "batch/v1",
							"kind":       "Job",
							"metadata": map[string]interface{}{
								"name":      "somehook",
								"namespace": "bar",
								"annotations": map[string]interface{}{
									meta.AnnotationHookType:         hook.TypePreApply,
									meta.AnnotationHookAllowFailure: "true",
									meta.AnnotationHookWaitTimeout:  "1h",
								},
								"labels": map[string]interface{}{
									meta.LabelHookChartName: "foochart",
									meta.LabelHookType:      hook.TypePreApply,
								},
								"uid": "some-uid",
							},
							"spec": map[string]interface{}{
								"template": map[string]interface{}{
									"spec": map[string]interface{}{
										"restartPolicy": "Never",
									},
								},
							},
						},
					}),
				},
			},
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}

				if actions[0].GetVerb() != "list" {
					t.Error(spew.Sdump(actions))
				}

				if !actions[1].Matches("create", "jobs") {
					t.Error(spew.Sdump(actions))
				}

				obj := actions[1].(clienttesting.CreateAction).GetObject()

				metadata, err := kmeta.Accessor(obj)
				if err != nil {
					t.Fatal(err)
				}

				if metadata.GetName() != "somehook" {
					t.Fatalf("expected hook %q, got %q", "somehook", metadata.GetName())
				}
			},
			validateWaitRequests: func(t *testing.T, reqs []*wait.Request) {
				if len(reqs) != 1 {
					t.Fatal(spew.Sdump(reqs))
				}

				if len(reqs[0].Visitor.(resource.InfoListVisitor)) != 1 {
					t.Fatal(spew.Sdump(reqs[0].Visitor))
				}

				if len(reqs[0].ResourceOptions) != 1 {
					t.Fatal(spew.Sdump(reqs[0].ResourceOptions))
				}

				require.Equal(t, 1*time.Hour, reqs[0].ResourceOptions["some-uid"].Timeout)
				require.True(t, reqs[0].ResourceOptions["some-uid"].AllowFailure)
			},
		},
		{
			name: "execute one hook with custom options, invalid timeout",
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				return dynamicfakeclient.NewSimpleDynamicClient(scheme.Scheme)
			},
			hookType: hook.TypePostApply,
			hooks: hook.Map{
				hook.TypePostApply: hook.List{
					hook.MustParse(&unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "batch/v1",
							"kind":       "Job",
							"metadata": map[string]interface{}{
								"name":      "somehook",
								"namespace": "bar",
								"annotations": map[string]interface{}{
									meta.AnnotationHookType:         hook.TypePostApply,
									meta.AnnotationHookAllowFailure: "true",
									meta.AnnotationHookWaitTimeout:  "0s",
								},
								"labels": map[string]interface{}{
									meta.LabelHookChartName: "foochart",
									meta.LabelHookType:      hook.TypePostApply,
								},
								"uid": "some-uid",
							},
							"spec": map[string]interface{}{
								"template": map[string]interface{}{
									"spec": map[string]interface{}{
										"restartPolicy": "Never",
									},
								},
							},
						},
					}),
				},
			},
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}

				if actions[0].GetVerb() != "list" {
					t.Error(spew.Sdump(actions))
				}

				if !actions[1].Matches("create", "jobs") {
					t.Error(spew.Sdump(actions))
				}
			},
			validateWaitRequests: func(t *testing.T, reqs []*wait.Request) {
				if len(reqs) != 1 {
					t.Fatal(spew.Sdump(reqs))
				}

				if len(reqs[0].ResourceOptions) != 1 {
					t.Fatal(spew.Sdump(reqs[0].ResourceOptions))
				}

				require.Equal(t, wait.DefaultWaitTimeout, reqs[0].ResourceOptions["some-uid"].Timeout)
			},
		},
		{
			name: "no hooks executed during dry-run",
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				return dynamicfakeclient.NewSimpleDynamicClient(scheme.Scheme)
			},
			dryRun:   true,
			hookType: hook.TypePostApply,
			hooks: hook.Map{
				hook.TypePostApply: hook.List{
					hook.MustParse(&unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "batch/v1",
							"kind":       "Job",
							"metadata": map[string]interface{}{
								"name":      "somehook",
								"namespace": "bar",
								"annotations": map[string]interface{}{
									meta.AnnotationHookType: hook.TypePostApply,
								},
								"labels": map[string]interface{}{
									meta.LabelHookChartName: "foochart",
									meta.LabelHookType:      hook.TypePostApply,
								},
							},
							"spec": map[string]interface{}{
								"template": map[string]interface{}{
									"spec": map[string]interface{}{
										"restartPolicy": "Never",
									},
								},
							},
						},
					}),
				},
			},
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 1 {
					t.Fatal(spew.Sdump(actions))
				}

				if actions[0].GetVerb() != "list" {
					t.Error(spew.Sdump(actions))
				}
			},
			validateDeletions: func(t *testing.T, deleter *deletions.FakeDeleter) {
				if len(deleter.Infos) != 0 {
					t.Fatal(spew.Sdump(deleter.Infos))
				}
			},
			validateWaitRequests: func(t *testing.T, reqs []*wait.Request) {
				if len(reqs) != 0 {
					t.Fatal(spew.Sdump(reqs))
				}
			},
		},
		{
			name: "no hooks defined",
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				return dynamicfakeclient.NewSimpleDynamicClient(scheme.Scheme)
			},
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 0 {
					t.Fatal(spew.Sdump(actions))
				}
			},
			validateWaitRequests: func(t *testing.T, reqs []*wait.Request) {
				if len(reqs) != 0 {
					t.Fatal(spew.Sdump(reqs))
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeClient := tc.fakeClient()
			deleter := deletions.NewFakeDeleter()
			waiter := wait.NewFakeWaiter()

			e := &HookExecutor{
				IOStreams:     genericclioptions.NewTestIOStreamsDiscard(),
				Deleter:       deleter,
				Waiter:        waiter,
				Mapper:        testrestmapper.TestOnlyStaticRESTMapper(scheme.Scheme),
				DynamicClient: fakeClient,
				Printer:       printers.NewDiscardingContextPrinter(),
				DryRun:        tc.dryRun,
			}

			err := e.ExecHooks(newTestChart(tc.hooks), tc.hookType)
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

			if tc.validateActions != nil {
				tc.validateActions(t, fakeClient.Actions())
			}

			if tc.validateDeletions != nil {
				tc.validateDeletions(t, deleter)
			}

			if tc.validateWaitRequests != nil {
				tc.validateWaitRequests(t, waiter.Requests)
			}
		})
	}
}

func TestHookExecutor_ExecHooks_Nil(t *testing.T) {
	var executor *HookExecutor

	assert.NoError(t, executor.ExecHooks(&Chart{}, hook.TypePreApply))
}

func newTestChart(hooks hook.Map) *Chart {
	return &Chart{
		Config: &Config{
			Name: "foochart",
		},
		Hooks: hooks,
	}
}

func fakeClient() resource.FakeClientFunc {
	return func(version schema.GroupVersion) (resource.RESTClient, error) {
		return &fake.RESTClient{}, nil
	}
}

func fakeClientWith(testName string, t *testing.T, data map[string]string) resource.FakeClientFunc {
	return func(version schema.GroupVersion) (resource.RESTClient, error) {
		return &fake.RESTClient{
			GroupVersion:         schema.GroupVersion{Group: "apps", Version: "v1"},
			NegotiatedSerializer: serializer.DirectCodecFactory{CodecFactory: scheme.Codecs},
			Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
				p := req.URL.Path
				q := req.URL.RawQuery
				if len(q) != 0 {
					p = p + "?" + q
				}

				body, ok := data[p]
				if !ok {
					t.Fatalf("%s: unexpected request: %s (%s)\n%#v", testName, p, req.URL, req)
				}
				header := http.Header{}
				header.Set("Content-Type", runtime.ContentTypeJSON)
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     header,
					Body:       stringBody(body),
				}, nil
			}),
		}, nil
	}
}

func stringBody(body string) io.ReadCloser {
	return ioutil.NopCloser(bytes.NewReader([]byte(body)))
}

func newDefaultBuilder() *resource.Builder {
	return newDefaultBuilderWith(fakeClient())
}

func newDefaultBuilderWith(fakeClientFn resource.FakeClientFunc) *resource.Builder {
	return resource.NewFakeBuilder(
		fakeClientFn,
		func() (kmeta.RESTMapper, error) {
			return testrestmapper.TestOnlyStaticRESTMapper(scheme.Scheme), nil
		},
		func() (restmapper.CategoryExpander, error) {
			return resource.FakeCategoryExpander, nil
		})
}
