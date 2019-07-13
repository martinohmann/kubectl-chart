package deletions

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/cli-runtime/pkg/resource"
)

func TestFakeDeleter_Delete(t *testing.T) {
	d := NewFakeDeleter()

	infos1 := []*resource.Info{
		{Name: "foo"},
	}

	err := d.Delete(resource.InfoListVisitor(infos1))

	require.NoError(t, err)

	infos2 := []*resource.Info{
		{Name: "bar"},
	}

	err = d.Delete(resource.InfoListVisitor(infos2))

	require.NoError(t, err)

	expectedInfos := []*resource.Info{
		{Name: "foo"},
		{Name: "bar"},
	}

	assert.Equal(t, 2, d.Called)
	assert.Equal(t, expectedInfos, d.Infos)
}

type errorVisitor struct{}

func (*errorVisitor) Visit(fn resource.VisitorFunc) error {
	return errors.New("whoops")
}

func TestFakeDeleter_DeleteForwardVisitorErrors(t *testing.T) {
	d := NewFakeDeleter()

	err := d.Delete(&errorVisitor{})

	require.Error(t, err)
	assert.Equal(t, "whoops", err.Error())
}
