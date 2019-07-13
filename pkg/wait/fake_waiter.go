package wait

import "sync"

var _ Waiter = &FakeWaiter{}

type FakeWaiter struct {
	sync.Mutex
	Requests []*Request
}

func NewFakeWaiter() *FakeWaiter {
	return &FakeWaiter{
		Requests: make([]*Request, 0),
	}
}

func (d *FakeWaiter) Wait(r *Request) error {
	d.Lock()
	defer d.Unlock()

	d.Requests = append(d.Requests, r)

	return nil
}
