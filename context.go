package shutdown

import "context"

// ctxKey is a private struct used as a unique key for storing
// and retrieving the Closure value in the context.
type ctxKey struct{}

// ClosureToContext associates the provided Closure with the given context
// and returns a new context with that association.
func ClosureToContext(ctx context.Context, closure Closure) context.Context {
	return context.WithValue(ctx, ctxKey{}, closure)
}

// ClosureFromContext retrieves the Closure associated with the given context.
// It returns the Closure and a boolean indicating if the Closure was found.
func ClosureFromContext(ctx context.Context) (Closure, bool) {
	closure, ok := ctx.Value(ctxKey{}).(Closure)
	return closure, ok
}
