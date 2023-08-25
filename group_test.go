package shutdown

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type groupCloser struct {
	delay time.Duration
	err   error
	calls int32
}

func (m *groupCloser) Close() error {
	time.Sleep(m.delay)
	atomic.AddInt32(&m.calls, 1)
	return m.err
}

func TestGroupClose(t *testing.T) {
	g := &Group{}

	c1 := &groupCloser{delay: 10 * time.Millisecond}
	c2 := &groupCloser{delay: 20 * time.Millisecond, err: errors.New("closer error")}
	c3 := &groupCloser{delay: 30 * time.Millisecond}

	g.Append(c1)
	g.Append(c2)
	g.Append(c3)

	err := g.Close()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "closer error")
	assert.Equal(t, int32(1), c1.calls)
	assert.Equal(t, int32(1), c2.calls)
	assert.Equal(t, int32(1), c3.calls)
}

func TestGroupCloseContext(t *testing.T) {
	g := &Group{}

	c1 := &groupCloser{delay: 11 * time.Millisecond}
	c2 := &groupCloser{delay: 21 * time.Millisecond}
	c3 := &groupCloser{delay: 800 * time.Millisecond} // This will be interrupted

	g.Append(c1)
	g.Append(c2)
	g.Append(c3)

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()

	err := g.CloseContext(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&c1.calls))
	assert.Equal(t, int32(1), atomic.LoadInt32(&c2.calls))
	assert.Equal(t, int32(0), atomic.LoadInt32(&c3.calls)) // c3 should not be closed due to context timeout
}

func TestGroup_WithContext(t *testing.T) {
	// Create a new instance of Group
	g := &Group{}

	// Associate the Group instance with a new context
	ctx := g.WithContext(context.Background())

	// Retrieve the Group (as a Closure) from the context
	closure, ok := ClosureFromContext(ctx)

	// Assert that the retrieved closure is indeed our Group instance
	if !ok || closure != g {
		t.Fatalf("Expected to retrieve the original group from context, but got %v", closure)
	}
}
