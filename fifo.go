package shutdown

import (
	"context"
	"errors"
	"sync"
)

// Fifo closes registered resources in first-in, first-out order.
//
// The first resource appended to the manager is the first resource closed during
// shutdown. This strategy is useful when resources were acquired in the same
// order they should be released, or when shutdown ordering should mirror the
// application startup sequence rather than unwind it.
//
// Fifo is safe for concurrent Append and Close calls. Shutdown itself is
// idempotent: the first call to Close or CloseContext performs the work, and
// subsequent calls return the previously computed result.
type Fifo struct {
	mu     sync.Mutex
	once   sync.Once
	closed bool
	result error
	queue  []Closer
}

// NewFIFO constructs an empty FIFO shutdown manager.
//
// The returned value is ready for immediate use and does not require further
// initialization.
func NewFIFO() *Fifo {
	return &Fifo{}
}

// Append registers a closer to be executed during shutdown.
//
// Nil closers are ignored. If shutdown has already started, Append panics,
// because adding resources after the shutdown sequence has been fixed is treated
// as a programming error in the current API.
func (f *Fifo) Append(closer Closer) {
	if closer == nil {
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		panic("shutdown: append after close")
	}

	f.queue = append(f.queue, closer)
}

// CloseContext starts FIFO shutdown using the supplied context.
//
// Each registered closer is executed in append order. Before running a new
// closer, the manager checks ctx.Err(); if the context has already been
// canceled, shutdown stops scheduling additional closers and the context error
// is joined into the returned error.
//
// If a closer implements ContextCloser, CloseContext forwards ctx to that
// method. Otherwise the manager falls back to Close.
//
// The first call performs shutdown and caches the result. Later calls return
// the cached result without re-running any closers.
//
// ctx must be non-nil. Passing nil is considered a caller bug and may panic.
func (f *Fifo) CloseContext(ctx context.Context) error {
	f.once.Do(func() {
		f.mu.Lock()
		f.closed = true
		closers := append([]Closer(nil), f.queue...)
		f.queue = nil
		f.mu.Unlock()

		errs := make([]error, 0, len(closers)+1)

		for _, closer := range closers {
			if ctxErr := ctx.Err(); ctxErr != nil {
				errs = appendContextError(errs, ctxErr)
				break
			}

			if err := closeWithContext(ctx, closer); err != nil {
				errs = append(errs, err)
			}
		}

		f.result = errors.Join(errs...)
	})

	return f.result
}

// Close starts FIFO shutdown with context.Background().
//
// It is equivalent to calling CloseContext(context.Background()).
func (f *Fifo) Close() error {
	return f.CloseContext(context.Background())
}
