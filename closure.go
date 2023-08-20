package shutdown

import (
	"context"
	"io"
	"sync"
)

// Closer is an alias for io.Closer. It represents an interface that requires a Close method.
type Closer = io.Closer

// Closure interface defines methods for appending and closing resources.
type Closure interface {
	Closer

	Append(closer Closer)                   // Appends a new closer
	CloseContext(ctx context.Context) error // Closes resources with context support
}

var (
	closure Closure    = &Lifo{} // Default implementation of Closure using Lifo (Last In First Out) strategy
	mu      sync.Mutex           // Mutex to ensure thread safety
)

// SetPackageClosure allows for setting a different Closure implementation.
func SetPackageClosure(c Closure) {
	mu.Lock()         // Acquiring the lock
	defer mu.Unlock() // Making sure to release the lock after the function exits
	closure = c       // Set the global closure to the provided implementation
}

// Append appends a new closer to the global closure.
func Append(closer Closer) {
	mu.Lock()              // Acquiring the lock
	defer mu.Unlock()      // Making sure to release the lock after the function exits
	closure.Append(closer) // Appending the closer
}

// Close attempts to close all appended resources.
func Close() error {
	mu.Lock()              // Acquiring the lock
	defer mu.Unlock()      // Making sure to release the lock after the function exits
	return closure.Close() // Close all resources and return any encountered error
}

// CloseContext attempts to close all appended resources with context support.
func CloseContext(ctx context.Context) error {
	mu.Lock()                        // Acquiring the lock
	defer mu.Unlock()                // Making sure to release the lock after the function exits
	return closure.CloseContext(ctx) // Close resources with context support and return any encountered error
}
