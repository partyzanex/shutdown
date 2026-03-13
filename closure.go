package shutdown

import (
	"context"
	"errors"
	"io"
)

// Closer is the minimal contract accepted by shutdown managers.
//
// It is declared as an alias of io.Closer so existing resources that already
// implement Close() error can be registered without adaptation.
//
// A manager treats each registered Closer as a single shutdown step. Depending
// on the concrete manager implementation, those steps may be executed
// sequentially or concurrently when shutdown starts.
type Closer = io.Closer

// ContextCloser is implemented by resources that support context-aware shutdown.
//
// When a registered resource implements ContextCloser, managers prefer
// CloseContext over Close during CloseContext execution. This allows the
// resource to observe cancellation, deadlines, and other context-derived
// shutdown policies.
//
// If a resource does not implement ContextCloser, managers fall back to the
// regular Close method defined by Closer.
type ContextCloser interface {
	CloseContext(ctx context.Context) error
}

// QuietCloser is a convenience contract for resources whose shutdown operation
// cannot fail.
//
// The compat layer can wrap a QuietCloser into a regular Closer by adapting its
// Close method to return a nil error.
type QuietCloser interface {
	Close()
}

// Manager is the common contract implemented by all shutdown strategies.
//
// A Manager owns a collection of registered closers and is responsible for
// invoking them exactly once when shutdown begins. Implementations differ in the
// order and concurrency model used during shutdown, but they share the same
// high-level lifecycle:
//
//  1. callers register resources with Append;
//  2. callers trigger shutdown with Close or CloseContext;
//  3. subsequent Close calls are idempotent and return the result of the first
//     shutdown attempt.
//
// Append is intentionally simple and does not return an error. In the current
// implementation, attempting to append after shutdown has begun is considered a
// programming error and panics.
//
// CloseContext requires a non-nil context. Passing nil is considered a caller
// bug and may panic inside a concrete manager implementation.
type Manager interface {
	Append(closer Closer)
	Close() error
	CloseContext(ctx context.Context) error
}

// Fn adapts a plain function to the Closer interface.
//
// Fn is useful when shutdown logic does not naturally live on a struct that
// implements io.Closer. It allows callers to register inline or delegated
// cleanup logic without creating a dedicated type.
//
// Example:
//
//	manager.Append(shutdown.Fn(func() error {
//		return server.Close()
//	}))
type Fn func() error

// Close calls the wrapped function and returns its result.
func (f Fn) Close() error {
	return f()
}

// ContextFn adapts a context-aware function to both Closer and ContextCloser.
//
// When a manager performs context-aware shutdown, it calls CloseContext and the
// wrapped function receives the caller-provided context. When a caller uses the
// plain Close method, ContextFn falls back to a background context.
//
// This adapter is useful for resources whose shutdown path needs deadline or
// cancellation information but does not warrant a custom type.
type ContextFn func(ctx context.Context) error

// Close invokes the wrapped function with context.Background().
//
// This preserves compatibility with the Closer interface when no explicit
// context is available.
func (f ContextFn) Close() error {
	return f.CloseContext(context.Background())
}

// CloseContext invokes the wrapped function with the provided context.
func (f ContextFn) CloseContext(ctx context.Context) error {
	return f(ctx)
}

// QuietFn adapts a no-error function to the Closer interface.
//
// It is the functional counterpart to QuietCloser and is useful for
// fire-and-forget cleanup logic that cannot fail.
type QuietFn func()

// Close invokes the wrapped function and always returns nil.
func (f QuietFn) Close() error {
	f()
	return nil
}

// closeWithContext executes a single shutdown step.
//
// The function prefers ContextCloser when available so that resources capable of
// observing deadlines and cancellation receive the caller-provided context.
// Otherwise it falls back to the standard Close method.
//
// A nil closer is treated as a no-op to keep manager implementations simple when
// they need to handle optional registrations.
func closeWithContext(ctx context.Context, closer Closer) error {
	if closer == nil {
		return nil
	}

	if contextCloser, ok := closer.(ContextCloser); ok {
		return contextCloser.CloseContext(ctx)
	}

	return closer.Close()
}

// appendContextError appends ctxErr to errs unless it is nil or already present.
//
// Managers use this helper when a shutdown loop stops because the supplied
// context has been canceled or its deadline has expired. The helper preserves
// the original slice if the context error is already represented in the
// accumulated error set, including inside an error produced by errors.Join.
func appendContextError(errs []error, ctxErr error) []error {
	if ctxErr == nil {
		return errs
	}

	for _, err := range errs {
		if errors.Is(err, ctxErr) {
			return errs
		}
	}

	return append(errs, ctxErr)
}
