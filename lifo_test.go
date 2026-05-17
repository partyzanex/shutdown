package shutdown

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLIFOClosesInReverseAppendOrder(t *testing.T) {
	lifo := NewLIFO()
	var order []string
	expected := errors.New("second failed")

	lifo.Append(Fn(func() error {
		order = append(order, "first")
		return nil
	}))
	lifo.Append(Fn(func() error {
		order = append(order, "second")
		return expected
	}))

	err := lifo.Close()
	require.Error(t, err)
	assert.ErrorIs(t, err, expected)

	assert.True(t, reflect.DeepEqual(order, []string{"second", "first"}), "unexpected close order: %v", order)
}

func TestLIFOUsesContextCloser(t *testing.T) {
	lifo := NewLIFO()
	expected := context.Canceled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	lifo.Append(ContextFn(func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}))

	err := lifo.CloseContext(ctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, expected)
}

func TestLIFOAppendIgnoresNilCloser(t *testing.T) {
	lifo := NewLIFO()

	assert.NotPanics(t, func() {
		lifo.Append(nil)
	})

	assert.Empty(t, lifo.stack)
}

func TestLIFORecoversPanicAndContinues(t *testing.T) {
	lifo := NewLIFO()
	first := &testCloser{}

	// LIFO: appended first → closed last. We want to ensure that a panic
	// in the most-recently-appended closer does not stop the earlier one.
	lifo.Append(first)
	lifo.Append(Fn(func() error { panic("boom") }))

	err := lifo.Close()
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPanic)
	assert.Equal(t, 1, first.called)
}

func TestLIFOTryAppend(t *testing.T) {
	t.Run("open manager accepts closer", func(t *testing.T) {
		lifo := NewLIFO()
		err := lifo.TryAppend(Fn(func() error { return nil }))
		require.NoError(t, err)
		assert.Len(t, lifo.stack, 1)
	})

	t.Run("closed manager returns ErrClosed without running closer", func(t *testing.T) {
		lifo := NewLIFO()
		require.NoError(t, lifo.Close())

		late := &testCloser{}
		err := lifo.TryAppend(late)
		require.ErrorIs(t, err, ErrClosed)
		assert.Equal(t, 0, late.called)
	})

	t.Run("nil closer is no-op", func(t *testing.T) {
		lifo := NewLIFO()
		assert.NoError(t, lifo.TryAppend(nil))
	})
}

func TestLIFOAppendAfterCloseRunsCloserInline(t *testing.T) {
	lifo := NewLIFO()
	require.NoError(t, lifo.Close())

	late := &testCloser{}

	require.NotPanics(t, func() {
		lifo.Append(late)
	})

	assert.Equal(t, 1, late.called, "late closer must be closed inline")
	assert.Empty(t, lifo.stack, "late closer must not be stored")
}
