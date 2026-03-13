package compat

import (
	"context"
	"errors"
	"os"
	"os/signal"
)

var errNoSignals = errors.New("shutdown/compat: at least one signal must be provided")

// WaitForSignal blocks until one of the specified signals is received or the
// context is done.
//
// If a matching signal is received first, it is returned together with a nil
// error. If the context finishes first, WaitForSignal returns nil and ctx.Err().
//
// The function registers signal delivery only for the duration of the call and
// always releases its subscription before returning. Calling WaitForSignal
// without specifying any signals returns an error.
//
// ctx must be non-nil. Passing nil is considered a caller bug and may panic.
func WaitForSignal(ctx context.Context, sig ...os.Signal) (os.Signal, error) {
	if len(sig) == 0 {
		return nil, errNoSignals
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, sig...)
	defer signal.Stop(c)

	select {
	case received := <-c:
		return received, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// CloseOnSignal waits for a signal and then shuts down the shared compat
// manager.
//
// waitCtx controls how long the function waits for a signal. shutdownCtx is
// passed to CloseContext after a signal has been received. Separating these
// contexts allows callers to cancel signal waiting without accidentally forcing
// shutdown to reuse a canceled context.
//
// Both contexts must be non-nil. Passing nil is considered a caller bug and may
// panic.
func CloseOnSignal(waitCtx, shutdownCtx context.Context, sig ...os.Signal) error {
	if _, err := WaitForSignal(waitCtx, sig...); err != nil {
		return err
	}

	return CloseContext(shutdownCtx)
}
