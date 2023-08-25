package shutdown

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type pkgCloser struct {
	mu      sync.Mutex
	isClose bool
	err     error
}

func (mc *pkgCloser) Close() error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.isClose = true
	return mc.err
}

func TestAppendAndClose(t *testing.T) {
	SetPackageClosure(&Lifo{})
	once = sync.Once{}
	mCloser := &pkgCloser{}
	Append(mCloser)
	if err := Close(); err != nil || !mCloser.isClose {
		t.Fatalf("Expected closer to be closed without errors, got: %v", err)
	}
}

func TestAppendAndCloseWithError(t *testing.T) {
	SetPackageClosure(&Fifo{})
	once = sync.Once{}
	expectedErr := errors.New("close error")
	mCloser := &pkgCloser{err: expectedErr}
	Append(mCloser)
	if err := Close(); err == nil || err.Error() != expectedErr.Error() {
		t.Fatalf("Expected error: %v, got: %v", expectedErr, err)
	}
}

type mockLogger struct {
	messages []string
	mu       sync.Mutex
}

func (ml *mockLogger) Msgf(format string, args ...interface{}) {
	ml.mu.Lock()
	defer ml.mu.Unlock()

	ml.messages = append(ml.messages, fmt.Sprintf(format, args...))
}

func TestWaitForSignals(t *testing.T) {
	ml := &mockLogger{}
	signals := []os.Signal{os.Interrupt}

	go func() {
		// Simulate a signal after a short delay
		time.Sleep(100 * time.Millisecond)
		process, _ := os.FindProcess(os.Getpid())
		_ = process.Signal(os.Interrupt)
	}()

	WaitForSignals(ml, signals...)

	if len(ml.messages) == 0 || ml.messages[0] != "Received signal: interrupt" {
		t.Errorf("Expected log message about received signal, got: %v", ml.messages)
	}
}

func TestWaitForSignalsContext(t *testing.T) {
	ml := &mockLogger{}
	signals := []os.Signal{os.Interrupt}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	WaitForSignalsContext(ctx, ml, signals...)

	if len(ml.messages) == 0 || ml.messages[0] != "Received signal: context deadline exceeded" {
		t.Errorf("Expected log message about context deadline, got: %v", ml.messages)
	}
}

func TestFn_Close(t *testing.T) {
	c := Fn(func() error {
		return nil
	})
	if err := c.Close(); err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

func getLastLoggedMessage(ml *mockLogger) string {
	ml.mu.Lock()
	defer ml.mu.Unlock()

	if len(ml.messages) == 0 {
		return ""
	}

	return ml.messages[len(ml.messages)-1]
}

func TestCloseOnSignal(t *testing.T) {
	logger := &mockLogger{}

	go func() {
		// Simulate a signal after a short delay
		time.Sleep(100 * time.Millisecond)
		process, _ := os.FindProcess(os.Getpid())
		_ = process.Signal(os.Interrupt)
	}()

	// Normally, you'd want to simulate the actual os.Signal here, but this example
	// just checks if the logger logs the message.
	err := CloseOnSignal(logger, os.Interrupt)
	assert.NoError(t, err)

	assert.Equal(t, "Received signal: interrupt", getLastLoggedMessage(logger))
}

func TestCloseOnSignalContextCancelled(t *testing.T) {
	logger := &mockLogger{}
	ctx, cancel := context.WithCancel(context.Background())

	// Simulate context completion without waiting for the actual os.Signal.
	go func() {
		cancel()
	}()

	err := CloseOnSignalContext(ctx, logger, os.Interrupt)
	assert.NoError(t, err)

	assert.Equal(t, "Received signal: context canceled", getLastLoggedMessage(logger))
}
