package shutdown

import (
	"context"
	"io"
	"os"
	"os/signal"
	"sync"
)

// Closer is an alias for io.Closer. It represents an interface that requires a Close method.
type Closer = io.Closer

// Closure interface defines methods for appending and closing resources.
type Closure interface {
	Closer

	Append(closer Closer)                            // Appends a new closer
	CloseContext(ctx context.Context) error          // Closes resources with context support
	WithContext(ctx context.Context) context.Context // Sets the context for the closure
}

var (
	pkgClosure Closure    = &Lifo{} // Default implementation of Closure using Lifo (Last In First Out) strategy
	mu         sync.Mutex           // Mutex to ensure thread safety
	once       sync.Once
)

// SetPackageClosure allows for setting a different Closure implementation.
func SetPackageClosure(c Closure) {
	mu.Lock()         // Acquiring the lock
	defer mu.Unlock() // Making sure to release the lock after the function exits
	pkgClosure = c    // Set the global closure to the provided implementation
}

// Append appends a new closer to the global closure.
func Append(closer Closer) {
	mu.Lock()                 // Acquiring the lock
	defer mu.Unlock()         // Making sure to release the lock after the function exits
	pkgClosure.Append(closer) // Appending the closer
}

// Close attempts to close all appended resources.
func Close() error {
	return CloseContext(context.Background()) // Close all resources and return any encountered error
}

// CloseContext attempts to close all appended resources with context support.
func CloseContext(ctx context.Context) error {
	mu.Lock()         // Acquiring the lock
	defer mu.Unlock() // Making sure to release the lock after the function exits

	var err error

	once.Do(func() {
		err = pkgClosure.CloseContext(ctx) // Close all resources and return any encountered error
	})

	return err
}

// Logger is an interface representing logging capabilities. It provides a method to log warning messages.
type Logger interface {
	Msgf(format string, args ...interface{})
}

// WaitForSignals blocks until a given signal (or signals) is received.
// Once the signal is caught, it logs a warning message using the provided logger.
func WaitForSignals(logger Logger, sig ...os.Signal) {
	// Create a channel to listen for signals.
	c := make(chan os.Signal, 1)

	// Register the given signals to the channel.
	signal.Notify(c, sig...)

	// Ensure that we stop the signal notifications to the channel when the function returns.
	defer signal.Stop(c)

	// Log a warning when a signal is received.
	logger.Msgf("Received signal: %s", <-c)
}

// WaitForSignalsContext is similar to WaitForSignals but with support for context.
// It blocks until a given signal (or signals) is received or the context is done.
func WaitForSignalsContext(ctx context.Context, logger Logger, sig ...os.Signal) {
	// Create a context that will be done when the given signals are caught or the parent context is done.
	sigCtx, cancel := signal.NotifyContext(ctx, sig...)

	// Ensure resources are released when the function returns.
	defer cancel()

	// Wait until the signal context is done (either from a caught signal or the parent context).
	<-sigCtx.Done()

	// Log a warning indicating which signal or context-related error occurred.
	logger.Msgf("Received signal: %s", sigCtx.Err())
}

type Fn func() error

func (f Fn) Close() error {
	return f()
}

// CloseOnSignal waits for the specified signals and then closes the global closure.
// It utilizes the WaitForSignals function to wait for the signals.
// Once a signal is received, it will close the global closure using the Close function.
// The Logger parameter is used to log the received signal.
//
// Parameters:
// - logger: An instance that implements the Logger interface, used for logging.
// - sig: A variable list of os.Signal values that the function should wait for.
//
// Returns:
// - An error if encountered while closing the global closure; otherwise, nil.
func CloseOnSignal(logger Logger, sig ...os.Signal) error {
	WaitForSignals(logger, sig...)

	return Close()
}

// CloseOnSignalContext is similar to CloseOnSignal but with support for context.
// It waits for the specified signals or until the context is done, then closes the global closure.
// It utilizes the WaitForSignalsContext function to wait for the signals with context support.
// Once a signal is received or the context is done, it will close the global closure using the CloseContext function.
//
// Parameters:
// - ctx: The context that can be used to cancel or time out the waiting process.
// - logger: An instance that implements the Logger interface, used for logging.
// - sig: A variable list of os.Signal values that the function should wait for.
//
// Returns:
// - An error if encountered while closing the global closure; otherwise, nil.
func CloseOnSignalContext(ctx context.Context, logger Logger, sig ...os.Signal) error {
	WaitForSignalsContext(ctx, logger, sig...)

	return CloseContext(ctx)
}
