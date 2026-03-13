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
	mu      sync.Mutex
	once    sync.Once
	closed  bool
	result  error
	closers []Closer
}

// NewGroup constructs an empty concurrent shutdown manager.
func NewGroup() *Group {
	return &Group{}
}

// Append registers a closer to be executed during shutdown.
//
// Nil closers are ignored. Appending after shutdown has started panics.
func (g *Group) Append(closer Closer) {
	if closer == nil {
		return
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	if g.closed {
		panic("shutdown: append after close")
	}

	g.closers = append(g.closers, closer)
}

// CloseContext starts concurrent shutdown using the supplied context.
//
// All registered closers are launched concurrently. If a closer implements
// ContextCloser, it receives the supplied context; otherwise Close is used.
//
// Group differs intentionally from the sequential managers: it waits for all
// started closers to finish even if ctx becomes done while shutdown is running.
// After all goroutines complete, any observed closer errors are joined together,
// and the context error is appended if present.
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

		errs := make([]error, len(closers))
		var wg sync.WaitGroup

		wg.Add(len(closers))
		for i, closer := range closers {
			go func(index int, closer Closer) {
				defer wg.Done()
				errs[index] = closeWithContext(ctx, closer)
			}(i, closer)
		}

		wg.Wait()
		errs = appendContextError(errs, ctx.Err())
		g.result = errors.Join(errs...)
	})

	return g.result
}

// Close starts concurrent shutdown with context.Background().
func (g *Group) Close() error {
	return g.CloseContext(context.Background())
}
