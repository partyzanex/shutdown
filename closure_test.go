package shutdown

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testContextCloser struct {
	closeCalled        int
	closeContextCalled int
	lastContext        context.Context
	closeErr           error
	closeContextErr    error
}

func (c *testContextCloser) Close() error {
	c.closeCalled++
	return c.closeErr
}

func (c *testContextCloser) CloseContext(ctx context.Context) error {
	c.closeContextCalled++
	c.lastContext = ctx
	return c.closeContextErr
}

type testCloser struct {
	called int
	err    error
}

func (c *testCloser) Close() error {
	c.called++
	return c.err
}

func TestFnClose(t *testing.T) {
	expected := errors.New("fn failed")
	calls := 0

	closer := Fn(func() error {
		calls++
		return expected
	})

	err := closer.Close()
	require.Error(t, err)
	assert.ErrorIs(t, err, expected)
	assert.Equal(t, 1, calls)
}

func TestContextFnCloseUsesBackgroundContext(t *testing.T) {
	calls := 0
	closer := ContextFn(func(ctx context.Context) error {
		calls++
		assert.NotNil(t, ctx)
		assert.NoError(t, ctx.Err())
		return nil
	})

	err := closer.Close()
	require.NoError(t, err)
	assert.Equal(t, 1, calls)
}

func TestContextFnCloseContextPassesProvidedContext(t *testing.T) {
	expected := context.Canceled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	calls := 0
	closer := ContextFn(func(got context.Context) error {
		calls++
		assert.Same(t, ctx, got)
		return got.Err()
	})

	err := closer.CloseContext(ctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, expected)
	assert.Equal(t, 1, calls)
}

func TestQuietFnClose(t *testing.T) {
	calls := 0
	closer := QuietFn(func() {
		calls++
	})

	err := closer.Close()
	require.NoError(t, err)
	assert.Equal(t, 1, calls)
}

type testCtxKey struct{}

func TestCloseWithContext(t *testing.T) {
	expected := errors.New("context close failed")
	ctx := context.WithValue(context.Background(), testCtxKey{}, "marker")

	t.Run("nil closer", func(t *testing.T) {
		assert.NoError(t, closeWithContext(ctx, nil))
	})

	t.Run("context closer uses CloseContext", func(t *testing.T) {
		closer := &testContextCloser{closeErr: errors.New("plain close"), closeContextErr: expected}

		err := closeWithContext(ctx, closer)
		require.Error(t, err)
		assert.ErrorIs(t, err, expected)
		assert.Equal(t, 0, closer.closeCalled)
		assert.Equal(t, 1, closer.closeContextCalled)
		assert.Same(t, ctx, closer.lastContext)
	})

	t.Run("plain closer falls back to Close", func(t *testing.T) {
		closer := &testCloser{err: expected}

		err := closeWithContext(ctx, closer)
		require.Error(t, err)
		assert.ErrorIs(t, err, expected)
		assert.Equal(t, 1, closer.called)
	})
}

func TestWithErrHandler(t *testing.T) {
	closeErr := errors.New("close failed")

	t.Run("LIFO calls handler when inline-close errors", func(t *testing.T) {
		var got error
		lifo := NewLIFO(WithErrHandler(func(err error) { got = err }))
		require.NoError(t, lifo.Close())

		lifo.Append(Fn(func() error { return closeErr }))
		assert.ErrorIs(t, got, closeErr)
	})

	t.Run("FIFO calls handler when inline-close errors", func(t *testing.T) {
		var got error
		fifo := NewFIFO(WithErrHandler(func(err error) { got = err }))
		require.NoError(t, fifo.Close())

		fifo.Append(Fn(func() error { return closeErr }))
		assert.ErrorIs(t, got, closeErr)
	})

	t.Run("Group calls handler when inline-close errors", func(t *testing.T) {
		var got error
		group := NewGroup(WithErrHandler(func(err error) { got = err }))
		require.NoError(t, group.Close())

		group.Append(Fn(func() error { return closeErr }))
		assert.ErrorIs(t, got, closeErr)
	})

	t.Run("no handler: nil error discarded silently", func(t *testing.T) {
		lifo := NewLIFO()
		require.NoError(t, lifo.Close())

		assert.NotPanics(t, func() {
			lifo.Append(Fn(func() error { return closeErr }))
		})
	})
}

func TestAppenderInterface(t *testing.T) {
	// Compile-time check: all three managers implement Appender.
	var _ Appender = NewLIFO()
	var _ Appender = NewFIFO()
	var _ Appender = NewGroup()

	lifo := NewLIFO()
	require.NoError(t, lifo.Close())

	var ap Appender = lifo
	err := ap.TryAppend(Fn(func() error { return nil }))
	assert.ErrorIs(t, err, ErrClosed)
}

func TestCloseWithContextRecoversPanic(t *testing.T) {
	ctx := context.Background()

	t.Run("plain closer panics with error value", func(t *testing.T) {
		boom := errors.New("boom")
		closer := Fn(func() error { panic(boom) })

		err := closeWithContext(ctx, closer)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrPanic)
		assert.ErrorIs(t, err, boom)
		assert.Contains(t, err.Error(), "goroutine", "stack trace must be included")
	})

	t.Run("context closer panics with string value", func(t *testing.T) {
		closer := ContextFn(func(context.Context) error { panic("kaboom") })

		err := closeWithContext(ctx, closer)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrPanic)
		assert.Contains(t, err.Error(), "kaboom")
		assert.Contains(t, err.Error(), "goroutine", "stack trace must be included")
	})

	t.Run("nil closer does not panic", func(t *testing.T) {
		assert.NoError(t, closeWithContext(ctx, nil))
	})
}

func TestAppendContextError(t *testing.T) {
	ctxErr := context.Canceled
	otherErr := errors.New("other")

	t.Run("nil context error keeps slice unchanged", func(t *testing.T) {
		errs := []error{otherErr}
		got := appendContextError(errs, nil)
		assert.Equal(t, errs, got)
	})

	t.Run("new context error is appended", func(t *testing.T) {
		errs := []error{otherErr}
		got := appendContextError(errs, ctxErr)
		require.Len(t, got, 2)
		assert.ErrorIs(t, errors.Join(got...), otherErr)
		assert.ErrorIs(t, errors.Join(got...), ctxErr)
	})

	t.Run("existing joined context error is not duplicated", func(t *testing.T) {
		errs := []error{errors.Join(otherErr, ctxErr)}
		got := appendContextError(errs, ctxErr)
		require.Len(t, got, 1)
		assert.Same(t, errs[0], got[0])
	})
}
