package shutdown

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFIFOClosesInAppendOrder(t *testing.T) {
	fifo := NewFIFO()
	var order []string
	expected := errors.New("second failed")

	fifo.Append(Fn(func() error {
		order = append(order, "first")
		return nil
	}))
	fifo.Append(Fn(func() error {
		order = append(order, "second")
		return expected
	}))

	err := fifo.Close()
	require.Error(t, err)
	assert.ErrorIs(t, err, expected)

	assert.True(t, reflect.DeepEqual(order, []string{"first", "second"}), "unexpected close order: %v", order)
}

func TestFIFOStopsSchedulingAfterContextCancellation(t *testing.T) {
	fifo := NewFIFO()
	called := false
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	fifo.Append(Fn(func() error {
		called = true
		return nil
	}))

	err := fifo.CloseContext(ctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	assert.False(t, called)
}

func TestFIFOIsIdempotent(t *testing.T) {
	fifo := NewFIFO()
	calls := 0
	expected := errors.New("boom")

	fifo.Append(Fn(func() error {
		calls++
		return expected
	}))

	first := fifo.Close()
	second := fifo.Close()

	require.Error(t, first)
	require.Error(t, second)
	assert.ErrorIs(t, first, expected)
	assert.ErrorIs(t, second, expected)
	assert.Equal(t, 1, calls)
}

func TestFIFOAppendIgnoresNilCloser(t *testing.T) {
	fifo := NewFIFO()

	assert.NotPanics(t, func() {
		fifo.Append(nil)
	})

	assert.Empty(t, fifo.queue)
}

func TestFIFOAppendPanicsAfterClose(t *testing.T) {
	fifo := NewFIFO()

	require.NoError(t, fifo.Close())
	require.PanicsWithValue(t, "shutdown: append after close", func() {
		fifo.Append(Fn(func() error { return nil }))
	})
}
