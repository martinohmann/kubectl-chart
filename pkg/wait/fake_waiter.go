package wait

import "sync"

var _ Waiter = &FakeWaiter{}

type FakeWaiter struct {
	sync.Mutex
	Handler    func(r *Request) error
	CalledWith []*Request
}

func NewFakeWaiter(handler func(*Request) error) *FakeWaiter {
	return &FakeWaiter{
		Handler:    handler,
		CalledWith: make([]*Request, 0),
	}
}

func (d *FakeWaiter) Wait(r *Request) error {
	d.Lock()
	d.CalledWith = append(d.CalledWith, r)
	d.Unlock()

	if d.Handler == nil {
		return nil
	}

	return d.Handler(r)
}
