package shutdown

import (
	"context"
	"sync"

	"go.uber.org/multierr"
)

// Group represents a collection of resources that need to be closed.
type Group struct {
	closers []Closer   // The list of resources to close.
	mx      sync.Mutex // Mutex for thread safety.
}

// Append adds a new closer to the Group's list of closers.
func (g *Group) Append(closer Closer) {
	g.mx.Lock()         // Acquire the lock to ensure thread safety.
	defer g.mx.Unlock() // Release the lock after the function finishes.
	g.closers = append(g.closers, closer)
}

// CloseContext attempts to close each resource in the Group with context support.
// This allows external cancellation or timeout to be handled.
func (g *Group) CloseContext(ctx context.Context) error {
	g.mx.Lock()         // Acquire the lock to ensure thread safety.
	defer g.mx.Unlock() // Release the lock after the function finishes.

	// Prepare a slice to store errors from all the closers.
	var (
		errs = make([]error, 0, len(g.closers))
		mx   sync.Mutex // Local mutex for the error slice, to ensure thread safety while appending errors.
	)

	wg := sync.WaitGroup{} // WaitGroup to wait for all closers to finish.
	wg.Add(len(g.closers))

	// Iterate through each closer in the Group.
	for _, closer := range g.closers {
		go func(c Closer) {
			done := make(chan struct{}) // Channel to signal when the closer finishes.

			// Inner goroutine to call the Close method of the resource.
			go func() {
				if err := c.Close(); err != nil {
					mx.Lock()
					errs = append(errs, err) // If there's an error, append it to the errs slice.
					mx.Unlock()
				}

				close(done) // Signal that the closer is done.
			}()

			select {
			case <-ctx.Done(): // If the context is cancelled or times out.
			case <-done: // Wait until the closer finishes.
			}

			wg.Done() // Signal that this goroutine is finished.
		}(closer)
	}

	wg.Wait() // Wait until all closers are finished.

	// Combine all the errors into a single error using multierr.
	return multierr.Combine(errs...)
}

// Close attempts to close all resources in the Group without context support.
func (g *Group) Close() error {
	return g.CloseContext(context.Background()) // Use a default background context.
}

// WithContext associates the Group instance with the provided context.
// It uses the ClosureToContext function to embed the group into the context.
//
// Parameters:
// - ctx: The context to which the Group instance will be associated.
//
// Returns:
// - A new context containing the Group instance.
func (g *Group) WithContext(ctx context.Context) context.Context {
	return ClosureToContext(ctx, g)
}
