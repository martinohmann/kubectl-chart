package diff

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPathDiffer_Print(t *testing.T) {
	cases := []struct {
		name        string
		from, to    string
		expectedErr string
		expected    string
	}{
		{
			name:        "nonexistent file",
			from:        "testdata/dirone/configmap.yaml",
			to:          "testdata/dirtwo/nonexistent.yaml",
			expectedErr: "while creating path diff: stat testdata/dirtwo/nonexistent.yaml: no such file or directory",
		},
		{
			name: "diff two files",
			from: "testdata/dirone/configmap.yaml",
			to:   "testdata/dirtwo/configmap.yaml",
			expected: `--- configmap.yaml
+++ configmap.yaml
@@ -7 +7 @@
-  bar: baz
+  foo: bar
`,
		},
		{
			name: "diff two dirs with added file",
			from: "testdata/dirone",
			to:   "testdata/dirtwo",
			expected: `--- configmap.yaml
+++ configmap.yaml
@@ -7 +7 @@
-  bar: baz
+  foo: bar
--- <created>
+++ secret.yaml
@@ -0,0 +1,7 @@
+---
+apiVersion: v1
+kind: Secret
+metadata:
+  name: bar
+data:
+  baz: YmFyCg==
`,
		},
		{
			name: "diff two dirs with deleted file",
			from: "testdata/dirtwo",
			to:   "testdata/dirone",
			expected: `--- configmap.yaml
+++ configmap.yaml
@@ -7 +7 @@
-  foo: bar
+  bar: baz
--- secret.yaml
+++ <removed>
@@ -1,7 +0,0 @@
----
-apiVersion: v1
-kind: Secret
-metadata:
-  name: bar
-data:
-  baz: YmFyCg==
`,
		},
		{
			name: "diff dir with file",
			from: "testdata/dirone",
			to:   "testdata/dirtwo/configmap.yaml",
			expected: `--- configmap.yaml
+++ configmap.yaml
@@ -7 +7 @@
-  bar: baz
+  foo: bar
`,
		},
		{
			name: "diff file with dir",
			from: "testdata/dirone/configmap.yaml",
			to:   "testdata/dirtwo",
			expected: `--- configmap.yaml
+++ configmap.yaml
@@ -7 +7 @@
-  bar: baz
+  foo: bar
--- <created>
+++ secret.yaml
@@ -0,0 +1,7 @@
+---
+apiVersion: v1
+kind: Secret
+metadata:
+  name: bar
+data:
+  baz: YmFyCg==
`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := NewUnifiedPrinter(Options{})

			d := NewPathDiffer(tc.from, tc.to)

			var buf bytes.Buffer

			err := d.Print(p, &buf)

			if tc.expectedErr != "" {
				require.Error(t, err)

				assert.Equal(t, tc.expectedErr, err.Error())
			} else {
				require.NoError(t, err)

				assert.Equal(t, tc.expected, buf.String())
			}
		})
	}
}

func TestRemovalDiffer_Print(t *testing.T) {
	p := NewUnifiedPrinter(Options{})

	obj := &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mypod",
			Namespace: "foo",
		},
	}

	expected := `--- mypod
+++ <removed>
@@ -1,9 +0,0 @@
-apiVersion: v1
-kind: Pod
-metadata:
-  creationTimestamp: null
-  name: mypod
-  namespace: foo
-spec:
-  containers: null
-status: {}
`

	d := NewRemovalDiffer("mypod", obj)

	var buf bytes.Buffer

	err := d.Print(p, &buf)

	require.NoError(t, err)

	assert.Equal(t, expected, buf.String())
}
