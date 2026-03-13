package compat

import (
	"context"
	"sync"

	"github.com/partyzanex/shutdown"
)

var (
	mu      sync.RWMutex
	manager shutdown.Manager = shutdown.NewLIFO()
)

// Set replaces the shared manager used by the compat package.
//
// Set is intended for application bootstrap only, before the first call to
// Append, Close, or CloseContext. It does not migrate closers from the
// previously configured manager.
//
// Replacing the manager at runtime is allowed, but coordination of that change
// is the caller's responsibility. If Set races with Append or Close operations,
// some closers may remain registered on the previous manager and will not be
// closed by subsequent calls through compat.
func Set(m shutdown.Manager) {
	if m == nil {
		panic("shutdown/compat: nil manager")
	}

	mu.Lock()
	defer mu.Unlock()

	manager = m
}

// Append registers a closer on the shared compat manager.
//
// This is a package-level convenience wrapper around the currently configured
// shared manager. It is intended for applications that prefer singleton-style
// wiring over explicitly passing a manager instance through their startup code.
func Append(closer shutdown.Closer) {
	currentManager().Append(closer)
}

// AppendQuiet registers a quiet closer on the shared compat manager.
//
// The provided closer is adapted to shutdown.QuietFn so that it can be handled
// by the shared manager without introducing a separate error-returning wrapper.
// A nil quiet closer is ignored.
func AppendQuiet(closer shutdown.QuietCloser) {
	if closer == nil {
		return
	}

	currentManager().Append(shutdown.QuietFn(closer.Close))
}

// Close shuts down the shared compat manager with context.Background().
func Close() error {
	return currentManager().Close()
}

// CloseContext shuts down the shared compat manager using the supplied context.
//
// ctx must be non-nil. Passing nil is considered a caller bug and may panic in
// the configured manager.
func CloseContext(ctx context.Context) error {
	return currentManager().CloseContext(ctx)
}

// currentManager returns the manager currently configured for the compat layer.
//
// The returned value is a snapshot of the current shared manager reference.
// Callers should not assume it remains stable across future Set calls.
func currentManager() shutdown.Manager {
	mu.RLock()
	defer mu.RUnlock()

	return manager
}
