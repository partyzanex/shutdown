# Shutdown

`shutdown` is a small Go library for explicit, instance-based shutdown management.

The core package provides three strategies:

- `Lifo` for last-in, first-out shutdown
- `Fifo` for first-in, first-out shutdown
- `Group` for concurrent shutdown

## Requirements

- Go `1.25+`

## Design

The root package is instance-first: create a shutdown manager, append closers, then close it explicitly.

```go
manager := shutdown.NewLIFO()
manager.Append(db)
manager.Append(server)

ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

if err := manager.CloseContext(ctx); err != nil {
	log.Printf("shutdown failed: %v", err)
}
```

Package-level convenience APIs live in `shutdown/compat`. They exist for compatibility and simple wiring, but new code should prefer instance-based managers from the root package.

## Context semantics

`CloseContext` behaves as follows:

- if a closer implements `CloseContext(context.Context) error`, that method is used;
- otherwise the library falls back to `Close() error`;
- `Lifo` and `Fifo` stop scheduling new closers after context cancellation;
- `Group` closes resources concurrently and waits for all started closers to finish;
- errors are aggregated with `errors.Join`.

`CloseContext` requires a non-nil context. Passing `nil` is treated as a caller bug and may panic.

## Core API

```go
type ContextCloser interface {
	CloseContext(context.Context) error
}
```

Constructors:

- `shutdown.NewLIFO()`
- `shutdown.NewFIFO()`
- `shutdown.NewGroup()`

Helpers:

- `shutdown.Fn`
- `shutdown.ContextFn`
- `shutdown.QuietFn`

## Compatibility layer

For package-level convenience, use `shutdown/compat`:

```go
compat.Set(shutdown.NewLIFO())
compat.Append(server)
compat.Append(db)

waitCtx := context.Background()

shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

if err := compat.CloseOnSignal(waitCtx, shutdownCtx, os.Interrupt, syscall.SIGTERM); err != nil {
	log.Printf("compat shutdown failed: %v", err)
}
```

`compat.CloseOnSignal` accepts separate contexts for waiting and shutdown so cancellation of the wait phase does not corrupt shutdown execution.
`compat.WaitForSignal` requires at least one explicit signal.
All compat functions that accept `context.Context` also require a non-nil context.

## Notes

- repeated `Close` calls are idempotent and return the result of the first shutdown;
- appending after shutdown starts is invalid and will panic;
- `errors.Is` works with joined shutdown errors.
