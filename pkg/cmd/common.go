package cmd

import (
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
)

// DynamicClientGetter can be used to explicitly provide the dynamic REST
// client to use. It will automatically fall back to creating a new client if
// no client is provided.
type DynamicClientGetter struct {
	Client dynamic.Interface
}

// Get returns a dynamic REST client. If the Client field if the
// DynamicClientGetter is non-nil this will be returned, otherwise a dynamic
// client is built from the REST config obtained via the passed in
// RESTClientGetter.
func (g DynamicClientGetter) Get(f genericclioptions.RESTClientGetter) (dynamic.Interface, error) {
	if g.Client != nil {
		return g.Client, nil
	}

	config, err := f.ToRESTConfig()
	if err != nil {
		return nil, err
	}

	return dynamic.NewForConfig(config)
}
