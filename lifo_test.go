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

func TestLIFOAppendPanicsAfterClose(t *testing.T) {
	lifo := NewLIFO()

	require.NoError(t, lifo.Close())
	require.PanicsWithValue(t, "shutdown: append after close", func() {
		lifo.Append(Fn(func() error { return nil }))
	})
}
