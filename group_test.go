package shutdown

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroupReturnsPromptlyWhenContextIsCancelled(t *testing.T) {
	group := NewGroup()
	release := make(chan struct{})
	defer close(release)

	group.Append(Fn(func() error {
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
	case err := <-result:
		require.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("CloseContext must not wait for blocked closers when context is already done")
	}
}

func TestGroupHonorsDeadlineWhileClosersAreBlocked(t *testing.T) {
	group := NewGroup()
	release := make(chan struct{})
	defer close(release)

	group.Append(Fn(func() error {
		<-release
		return nil
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := group.CloseContext(ctx)
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, elapsed, 500*time.Millisecond,
		"CloseContext exceeded its deadline budget by a wide margin: %s", elapsed)
}

func TestGroupCollectsFinishedErrorsBeforeDeadline(t *testing.T) {
	group := NewGroup()
	release := make(chan struct{})
	defer close(release)
	fastErr := errors.New("fast failure")

	group.Append(Fn(func() error {
		return fastErr
	}))
	group.Append(Fn(func() error {
		<-release
		return nil
	}))

	// The fast closer returns immediately; 200ms is more than enough for the
	// library to record its error under the mutex before the deadline fires.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := group.CloseContext(ctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, fastErr)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
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

func TestGroupRecoversPanicAndRunsOtherClosers(t *testing.T) {
	group := NewGroup()
	other := &testCloser{}

	group.Append(Fn(func() error { panic("boom") }))
	group.Append(other)

	err := group.Close()
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPanic)
	assert.Equal(t, 1, other.called)
}

func TestGroupTryAppend(t *testing.T) {
	t.Run("open manager accepts closer", func(t *testing.T) {
		group := NewGroup()
		err := group.TryAppend(Fn(func() error { return nil }))
		require.NoError(t, err)
		assert.Len(t, group.closers, 1)
	})

	t.Run("closed manager returns ErrClosed without running closer", func(t *testing.T) {
		group := NewGroup()
		require.NoError(t, group.Close())

		late := &testCloser{}
		err := group.TryAppend(late)
		require.ErrorIs(t, err, ErrClosed)
		assert.Equal(t, 0, late.called)
	})

	t.Run("nil closer is no-op", func(t *testing.T) {
		group := NewGroup()
		assert.NoError(t, group.TryAppend(nil))
	})
}

func TestGroupAppendAfterCloseRunsCloserInline(t *testing.T) {
	group := NewGroup()
	require.NoError(t, group.Close())

	late := &testCloser{}

	require.NotPanics(t, func() {
		group.Append(late)
	})

	assert.Equal(t, 1, late.called, "late closer must be closed inline")
	assert.Empty(t, group.closers, "late closer must not be stored")
}
