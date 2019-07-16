package wait

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFakeWaiter_Wait(t *testing.T) {
	w := NewFakeWaiter()

	r1 := &Request{
		Options: &Options{Timeout: 1 * time.Second},
	}

	err := w.Wait(r1)

	require.NoError(t, err)

	r2 := &Request{}

	err = w.Wait(r2)

	require.NoError(t, err)

	expectedRequests := []*Request{r1, r2}

	assert.Equal(t, expectedRequests, w.Requests)
}
