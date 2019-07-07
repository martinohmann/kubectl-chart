package deletions

import "sync"

var _ Deleter = &FakeDeleter{}

type FakeDeleter struct {
	sync.Mutex
	Handler    func(r *Request) error
	CalledWith []*Request
}

func NewFakeDeleter(handler func(*Request) error) *FakeDeleter {
	return &FakeDeleter{
		Handler:    handler,
		CalledWith: make([]*Request, 0),
	}
}

func (d *FakeDeleter) Delete(r *Request) error {
	d.Lock()
	d.CalledWith = append(d.CalledWith, r)
	d.Unlock()

	if d.Handler == nil {
		return nil
	}

	return d.Handler(r)
}
