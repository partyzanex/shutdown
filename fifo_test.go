package shutdown

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type mockCloser struct {
	closeFunc func() error
}

func (m *mockCloser) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func TestFifo(t *testing.T) {
	t.Run("add and close without error", func(t *testing.T) {
		f := &Fifo{}
		closer1 := &mockCloser{}
		closer2 := &mockCloser{}

		f.Append(closer1)
		f.Append(closer2)

		err := f.Close()
		assert.Nil(t, err)
	})

	t.Run("add and close with error", func(t *testing.T) {
		f := &Fifo{}
		closer1 := &mockCloser{
			closeFunc: func() error {
				return errors.New("close error")
			},
		}
		closer2 := &mockCloser{}

		f.Append(closer1)
		f.Append(closer2)

		err := f.Close()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "close error")
	})

	t.Run("close with context cancel", func(t *testing.T) {
		f := &Fifo{}
		closer1 := &mockCloser{
			closeFunc: func() error {
				time.Sleep(100 * time.Millisecond)
				return nil
			},
		}
		closer2 := &mockCloser{}

		f.Append(closer1)
		f.Append(closer2)

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		err := f.CloseContext(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), context.DeadlineExceeded.Error())
	})
}

func TestFifo_WithContext(t *testing.T) {
	// Create a new Fifo instance
	fifo := &Fifo{}

	// Use the WithContext method to embed the Fifo instance into a new context.
	ctxWithFifo := fifo.WithContext(context.Background())

	// Extract the Closure (which should be our Fifo) from the context.
	extractedClosure, ok := ClosureFromContext(ctxWithFifo)

	// Assert that the Closure extracted from the context is indeed our Fifo instance.
	if !ok || extractedClosure != fifo {
		t.Fatalf("Expected to retrieve the original Fifo instance from context, but got %v", extractedClosure)
	}
}
