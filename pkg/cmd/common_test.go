package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfakeclient "k8s.io/client-go/dynamic/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
)

func TestDynamicClientGetter_Get(t *testing.T) {
	getter := DynamicClientGetter{}

	f := cmdtesting.NewTestFactory()

	client1, err := getter.Get(f)

	require.NoError(t, err)

	fakeClient := dynamicfakeclient.NewSimpleDynamicClient(runtime.NewScheme())

	getter.Client = fakeClient

	client2, err := getter.Get(f)

	require.NoError(t, err)

	require.NotEqual(t, client1, client2)
	require.Exactly(t, fakeClient, client2)
}
