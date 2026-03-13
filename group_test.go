package shutdown

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroupWaitsForClosersEvenWhenContextIsCancelled(t *testing.T) {
	group := NewGroup()
	release := make(chan struct{})
	done := make(chan struct{})

	group.Append(Fn(func() error {
		close(done)
		<-release
		return nil
	}))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := make(chan error, 1)
	go func() {
		result <- group.CloseContext(ctx)
	}()

	select {
	case <-result:
		t.Fatal("expected CloseContext to wait for running closers")
	case <-done:
	}

	close(release)

	select {
	case err := <-result:
		require.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for group shutdown to finish")
	}
}

func TestGroupJoinsErrors(t *testing.T) {
	group := NewGroup()
	errOne := errors.New("first")
	errTwo := errors.New("second")

	group.Append(Fn(func() error { return errOne }))
	group.Append(Fn(func() error { return errTwo }))

	err := group.Close()
	require.Error(t, err)
	assert.ErrorIs(t, err, errOne)
	assert.ErrorIs(t, err, errTwo)
}

func TestGroupAppendIgnoresNilCloser(t *testing.T) {
	group := NewGroup()

	assert.NotPanics(t, func() {
		group.Append(nil)
	})

	assert.Empty(t, group.closers)
}

func TestGroupAppendPanicsAfterClose(t *testing.T) {
	group := NewGroup()

	require.NoError(t, group.Close())
	require.PanicsWithValue(t, "shutdown: append after close", func() {
		group.Append(Fn(func() error { return nil }))
	})
}
