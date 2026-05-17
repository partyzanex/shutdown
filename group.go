package shutdown

import (
	"context"
	"errors"
	"sync"
)

// Group closes registered resources concurrently.
//
// Unlike Fifo and Lifo, Group does not impose an ordering relationship between
// registered closers. All closers are started in parallel during shutdown, and
// the manager waits for every started closer to finish before returning.
//
// Group is useful when shutdown steps are independent and serial execution would
// only increase total shutdown latency.
type Group struct {
	mu         sync.Mutex
	once       sync.Once
	closed     bool
	result     error
	closers    []Closer
	errHandler func(error)
}

// NewGroup constructs an empty concurrent shutdown manager.
func NewGroup(opts ...Option) *Group {
	o := applyOptions(opts)
	return &Group{errHandler: o.errHandler}
}

// Append registers a closer to be executed during shutdown.
//
// Nil closers are ignored. If shutdown has already started, the closer is
// closed inline via its Close method and any returned error is discarded.
// This makes Append safe to call from concurrent initialization paths that
// may race with an incoming signal.
func (g *Group) Append(closer Closer) {
	if closer == nil {
		return
	}

	g.mu.Lock()
	if g.closed {
		errHandler := g.errHandler
		g.mu.Unlock()

		if err := closer.Close(); err != nil && errHandler != nil {
			errHandler(err)
		}

		return
	}
	g.closers = append(g.closers, closer)
	g.mu.Unlock()
}

// TryAppend registers a closer to be executed during shutdown.
//
// Unlike Append, TryAppend returns ErrClosed instead of closing the resource
// inline when shutdown has already started. The caller decides what to do with
// the unregistered closer. Nil closers are ignored and nil is returned.
func (g *Group) TryAppend(closer Closer) error {
	if closer == nil {
		return nil
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	if g.closed {
		return ErrClosed
	}

	g.closers = append(g.closers, closer)

	return nil
}

// CloseContext starts concurrent shutdown using the supplied context.
//
// All registered closers are launched concurrently. If a closer implements
// ContextCloser, it receives the supplied context; otherwise Close is used.
//
// CloseContext honors ctx as a deadline: if ctx is done before every closer
// has finished, the call returns with the errors collected so far plus the
// context error joined in. Closers that have not yet completed are left
// running in the background; their eventual errors are discarded. This makes
// Group safe to use in shutdown paths that must respect a hard time budget.
//
// Shutdown is performed only once. Later calls return the cached result.
//
// ctx must be non-nil. Passing nil is considered a caller bug and may panic.
func (g *Group) CloseContext(ctx context.Context) error {
	g.once.Do(func() {
		g.mu.Lock()
		g.closed = true
		closers := append([]Closer(nil), g.closers...)
		g.closers = nil
		g.mu.Unlock()

		var (
			errsMu sync.Mutex
			errs   = make([]error, 0, len(closers)+1)
			wg     sync.WaitGroup
		)

		wg.Add(len(closers))
		for _, closer := range closers {
			go func(closer Closer) {
				defer wg.Done()
				if err := closeWithContext(ctx, closer); err != nil {
					errsMu.Lock()
					errs = append(errs, err)
					errsMu.Unlock()
				}
			}(closer)
		}

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-ctx.Done():
		}

		errsMu.Lock()
		snapshot := append([]error(nil), errs...)
		errsMu.Unlock()

		snapshot = appendContextError(snapshot, ctx.Err())
		g.result = errors.Join(snapshot...)
	})

	return g.result
}

// Close starts concurrent shutdown with context.Background().
func (g *Group) Close() error {
	return g.CloseContext(context.Background())
}
