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

func TestCloseWithContext(t *testing.T) {
	expected := errors.New("context close failed")
	ctx := context.WithValue(context.Background(), struct{}{}, "marker")

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
