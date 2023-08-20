package shutdown

import (
	"context"
	"sync"

	"go.uber.org/multierr"
)

// Fifo is a struct that manages a queue of resources that need to be closed, in First-In-First-Out order.
type Fifo struct {
	queue []Closer   // The list of resources to close
	mx    sync.Mutex // Mutex for thread safety
}

// Append adds a new closer to the end of the Fifo queue.
func (f *Fifo) Append(closer Closer) {
	f.mx.Lock()         // Acquiring the lock
	defer f.mx.Unlock() // Making sure to release the lock after the function exits
	f.queue = append(f.queue, closer)
}

// CloseContext attempts to close each resource in the Fifo queue with context support.
func (f *Fifo) CloseContext(ctx context.Context) error {
	f.mx.Lock()         // Acquiring the lock
	defer f.mx.Unlock() // Making sure to release the lock after the function exits

	var errs error // This will store the accumulated errors

	for _, closer := range f.queue {
		next := make(chan struct{}) // Channel to signal completion of the closer
		go func() {
			callClose(closer, &errs) // Call the close function and gather errors if any
			close(next)
		}()

		select {
		case <-ctx.Done():
			// If the context is cancelled or times out, return the accumulated errors and the context error
			return multierr.Append(errs, ctx.Err())
		case <-next:
			// Move to the next closer in the queue after the current one finishes
		}
	}

	return errs // Return the accumulated errors
}

// Close attempts to close all resources in the Fifo queue without context support.
func (f *Fifo) Close() error {
	return f.CloseContext(context.Background()) // Using a background context which will never be cancelled
}

// callClose safely calls the Close method of the given closer and appends any errors.
func callClose(closer Closer, errs *error) {
	if err := closer.Close(); err != nil {
		*errs = multierr.Append(*errs, err) // Accumulate the error if Close method fails
	}
}
