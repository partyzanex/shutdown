package shutdown

import (
	"context"
	"sync"

	"go.uber.org/multierr"
)

// Lifo represents a stack (Last-In, First-Out) of resources that need to be closed.
type Lifo struct {
	stack []Closer   // The stack of resources to close.
	mx    sync.Mutex // Mutex for thread safety.
}

// Append pushes a new closer onto the Lifo stack.
func (l *Lifo) Append(closer Closer) {
	l.mx.Lock()         // Acquire the lock to ensure thread safety.
	defer l.mx.Unlock() // Release the lock after the function finishes.
	l.stack = append(l.stack, closer)
}

// CloseContext attempts to close each resource in the Lifo stack with context support.
// It starts closing from the top of the stack (Last-In resource).
func (l *Lifo) CloseContext(ctx context.Context) error {
	l.mx.Lock()         // Acquire the lock to ensure thread safety.
	defer l.mx.Unlock() // Release the lock after the function finishes.

	var errs error // This will store the accumulated errors.

	// Start from the top of the stack and iterate in reverse order.
	for i := len(l.stack) - 1; i >= 0; i-- {
		next := make(chan struct{}) // Channel to signal completion of the closer.

		go func() {
			callClose(l.stack[i], &errs) // Call the close function for the current closer.
			close(next)
		}()

		select {
		case <-ctx.Done(): // If the context is cancelled or times out.
			return multierr.Append(errs, ctx.Err()) // Return the accumulated errors and the context error.
		case <-next: // Move to the next closer in the stack after the current one finishes.
		}
	}

	return errs // Return the accumulated errors.
}

// Close attempts to close all resources in the Lifo stack without context support.
func (l *Lifo) Close() error {
	return l.CloseContext(context.Background()) // Using a background context which will never be cancelled.
}

// WithContext embeds the Lifo instance into the given context.
// It utilizes the ClosureToContext function to associate the Lifo
// instance (as a Closure) with the provided context.
func (l *Lifo) WithContext(ctx context.Context) context.Context {
	return ClosureToContext(ctx, l)
}
