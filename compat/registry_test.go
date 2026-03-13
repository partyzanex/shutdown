package compat

import (
	"context"
	"errors"
	"testing"

	"github.com/partyzanex/shutdown"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type quietCloser struct {
	called int
}

func (c *quietCloser) Close() {
	c.called++
}

func TestRegistryClosesAssignedManager(t *testing.T) {
	lifecycle := shutdown.NewLIFO()
	Set(lifecycle)
	t.Cleanup(func() {
		Set(shutdown.NewLIFO())
	})

	expected := errors.New("boom")
	Append(shutdown.Fn(func() error { return expected }))

	err := Close()
	require.Error(t, err)
	assert.ErrorIs(t, err, expected)
}

func TestSetPanicsOnNilManager(t *testing.T) {
	require.PanicsWithValue(t, "shutdown/compat: nil manager", func() {
		Set(nil)
	})
}

func TestAppendQuietUsesSharedManager(t *testing.T) {
	lifecycle := shutdown.NewLIFO()
	Set(lifecycle)
	t.Cleanup(func() {
		Set(shutdown.NewLIFO())
	})

	closer := &quietCloser{}
	AppendQuiet(closer)

	require.NoError(t, Close())
	assert.Equal(t, 1, closer.called)
}

func TestAppendQuietIgnoresNilCloser(t *testing.T) {
	lifecycle := shutdown.NewLIFO()
	Set(lifecycle)
	t.Cleanup(func() {
		Set(shutdown.NewLIFO())
	})

	assert.NotPanics(t, func() {
		AppendQuiet(nil)
	})

	require.NoError(t, Close())
}

func TestCloseContextUsesSharedManager(t *testing.T) {
	lifecycle := shutdown.NewLIFO()
	Set(lifecycle)
	t.Cleanup(func() {
		Set(shutdown.NewLIFO())
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	Append(shutdown.ContextFn(func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}))

	err := CloseContext(ctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}
