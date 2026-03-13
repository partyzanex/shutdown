package compat

import (
	"context"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/partyzanex/shutdown"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWaitForSignalReturnsContextError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	sig, err := WaitForSignal(ctx, syscall.SIGUSR1)
	assert.Nil(t, sig)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestWaitForSignalRequiresExplicitSignals(t *testing.T) {
	sig, err := WaitForSignal(context.Background())
	assert.Nil(t, sig)
	require.Error(t, err)
	assert.ErrorIs(t, err, errNoSignals)
}

func TestWaitForSignalReturnsReceivedSignal(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	go func() {
		time.Sleep(10 * time.Millisecond)
		process, _ := os.FindProcess(os.Getpid())
		_ = process.Signal(syscall.SIGUSR1)
	}()

	sig, err := WaitForSignal(ctx, syscall.SIGUSR1)
	require.NoError(t, err)
	assert.Equal(t, syscall.SIGUSR1, sig)
}

func TestCloseOnSignalReturnsWaitError(t *testing.T) {
	waitCtx, cancel := context.WithCancel(context.Background())
	cancel()

	err := CloseOnSignal(waitCtx, context.Background(), syscall.SIGUSR1)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestCloseOnSignalClosesSharedManager(t *testing.T) {
	lifecycle := shutdown.NewLIFO()
	Set(lifecycle)
	t.Cleanup(func() {
		Set(shutdown.NewLIFO())
	})

	closed := false
	Append(shutdown.Fn(func() error {
		closed = true
		return nil
	}))

	waitCtx, waitCancel := context.WithTimeout(context.Background(), time.Second)
	defer waitCancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
	defer shutdownCancel()

	go func() {
		time.Sleep(10 * time.Millisecond)
		process, _ := os.FindProcess(os.Getpid())
		_ = process.Signal(syscall.SIGUSR1)
	}()

	err := CloseOnSignal(waitCtx, shutdownCtx, syscall.SIGUSR1)
	require.NoError(t, err)
	assert.True(t, closed)
}
