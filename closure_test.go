package shutdown

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

type pkgCloser struct {
	closeCalled bool
	err         error
}

func (mc *pkgCloser) Close() error {
	mc.closeCalled = true
	return mc.err
}

type mockClosure struct {
	appendCalled bool
	closeCalled  bool
	ctxCalled    bool
	closers      []Closer
}

func (mc *mockClosure) Append(closer Closer) {
	mc.appendCalled = true
	mc.closers = append(mc.closers, closer)
}

func (mc *mockClosure) Close() error {
	mc.closeCalled = true
	if len(mc.closers) > 0 {
		return mc.closers[0].Close()
	}
	return nil
}

func (mc *mockClosure) CloseContext(ctx context.Context) error {
	mc.ctxCalled = true
	if len(mc.closers) > 0 {
		return mc.closers[0].Close()
	}
	return nil
}

func TestPackageFunctions(t *testing.T) {
	// Reset package state after the test
	defer func() {
		SetPackageClosure(&Lifo{})
	}()

	// Mock closure for testing
	mc := &mockClosure{}
	SetPackageClosure(mc)

	// Test Append
	closer := &pkgCloser{}
	Append(closer)
	assert.True(t, mc.appendCalled, "Append was not called on the mock closure")
	assert.Contains(t, mc.closers, closer, "Closer was not added to the mock closure")

	// Test Close and CloseContext
	errClose := errors.New("test close error")
	closer.err = errClose

	err := Close()
	assert.True(t, mc.closeCalled, "Close was not called on the mock closure")
	assert.Equal(t, errClose, err, "Unexpected error returned from Close")

	err = CloseContext(context.Background())
	assert.True(t, mc.ctxCalled, "CloseContext was not called on the mock closure")
	assert.Equal(t, errClose, err, "Unexpected error returned from CloseContext")
}
