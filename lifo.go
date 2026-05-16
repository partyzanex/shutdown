package shutdown

import (
	"context"
	"errors"
	"sync"
)

// Lifo closes registered resources in last-in, first-out order.
//
// The most recently appended resource is closed first. This is the classic
// stack-like shutdown strategy and is often a good fit when resources should be
// unwound in reverse acquisition order.
//
// Lifo is safe for concurrent use and performs shutdown at most once.
type Lifo struct {
	mu         sync.Mutex
	once       sync.Once
	closed     bool
	result     error
	stack      []Closer
	errHandler func(error)
}

// NewLIFO constructs an empty LIFO shutdown manager.
func NewLIFO(opts ...Option) *Lifo {
	o := applyOptions(opts)
	return &Lifo{errHandler: o.errHandler}
}

// Append registers a closer to be executed during shutdown.
//
// Nil closers are ignored. If shutdown has already started, the closer is
// closed inline via its Close method and any returned error is discarded.
// This makes Append safe to call from concurrent initialization paths that
// may race with an incoming signal.
func (l *Lifo) Append(closer Closer) {
	if closer == nil {
		return
	}

	l.mu.Lock()
	if l.closed {
		errHandler := l.errHandler
		l.mu.Unlock()

		if err := closer.Close(); err != nil && errHandler != nil {
			errHandler(err)
		}

		return
	}
	l.stack = append(l.stack, closer)
	l.mu.Unlock()
}

// TryAppend registers a closer to be executed during shutdown.
//
// Unlike Append, TryAppend returns ErrClosed instead of closing the resource
// inline when shutdown has already started. The caller decides what to do with
// the unregistered closer. Nil closers are ignored and nil is returned.
func (l *Lifo) TryAppend(closer Closer) error {
	if closer == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return ErrClosed
	}

	l.stack = append(l.stack, closer)

	return nil
}

// CloseContext starts LIFO shutdown using the supplied context.
//
// Registered closers are executed in reverse append order. The manager checks
// ctx.Err() before scheduling the next closer; if the context has been
// canceled, remaining closers are skipped and the context error is joined into
// the final result.
//
// Context-aware closers receive the supplied context through CloseContext.
// Plain closers are executed through Close.
//
// Shutdown is idempotent: only the first call executes closers.
//
// ctx must be non-nil. Passing nil is considered a caller bug and may panic.
func (l *Lifo) CloseContext(ctx context.Context) error {
	l.once.Do(func() {
		l.mu.Lock()
		l.closed = true
		closers := append([]Closer(nil), l.stack...)
		l.stack = nil
		l.mu.Unlock()

		errs := make([]error, 0, len(closers)+1)

		for i := len(closers) - 1; i >= 0; i-- {
			if ctxErr := ctx.Err(); ctxErr != nil {
				errs = appendContextError(errs, ctxErr)
				break
			}

			if err := closeWithContext(ctx, closers[i]); err != nil {
				errs = append(errs, err)
			}
		}

		l.result = errors.Join(errs...)
	})

	return l.result
}

// Close starts LIFO shutdown with context.Background().
func (l *Lifo) Close() error {
	return l.CloseContext(context.Background())
}
