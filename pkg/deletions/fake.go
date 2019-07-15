package deletions

import (
	"sync"

	"k8s.io/cli-runtime/pkg/resource"
)

var _ Deleter = &FakeDeleter{}

type FakeDeleter struct {
	sync.Mutex
	Infos  []*resource.Info
	Called int
}

func NewFakeDeleter() *FakeDeleter {
	return &FakeDeleter{
		Infos: []*resource.Info{},
	}
}

func (d *FakeDeleter) Delete(v resource.Visitor) error {
	d.Lock()
	defer d.Unlock()

	d.Called++

	return v.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}

		d.Infos = append(d.Infos, info)

		return nil
	})
}
