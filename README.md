# Shutdown

The Shutdown provides a structured approach to managing and safely closing resources in a Go application.
It offers a variety of strategies to handle different ordering preferences for closing resources such as
**FIFO** (First-In, First-Out), **LIFO** (Last-In, First-Out), and **group**-based closings.

## Features

* **Concurrency Safe:** All operations are made concurrency safe using mutex locks.
* **Context Support:** Allows you to close resources with context support. This is useful for timeouts or external cancellation.
* **Error Aggregation:** Combines errors from multiple closers into a single error using the [go.uber.org/multierr](go.uber.org/multierr) library.

## Components

### Closer

An alias for `io.Closer` interface which requires a Close method.

### FIFO (First-In, First-Out)

Fifo struct manages a queue of resources to be closed in the order they were added.

### LIFO (Last-In, First-Out)

Lifo struct manages a stack of resources, ensuring they are closed in the reverse order they were added.

### Group

Group struct manages a collection of resources that need to be closed. It spawns a goroutine 
for each closer to ensure they close concurrently.

## Installation

Make sure you have Go installed and use:

```shell
go get github.com/partyzanex/shutdown@latest
```

## Usage

### Appending Closers:

Here's an example showcasing the **Lifo** strategy, where resources are added to a 
stack and then closed in a Last-In, First-Out manner:

```go
type MyCloser struct{}

func (c *MyCloser) Close() error {
    return errors.New("my closer error")
}
```

now we can add our closers to the stack:

```go
lifo := new(Lifo)
closer1 := &MyCloser{}
closer2 := io.NopCloser(nil)

lifo.Append(closer1)
lifo.Append(closer2)

err := lifo.CloseContext(context.Background())
fmt.Println(err)
// Output: my closer error
```

### Closing Resources with Context:

Employ the CloseContext method to facilitate resource shutdown with context backing:

```go
err := lifoCloser.CloseContext(ctx)
```

### Closing Resources without Context:

To terminate resources absent context support, use the Close method:

```go
err := lifoCloser.Close()
```

## Dependencies

* [go.uber.org/multierr](go.uber.org/multierr): A dependency used for amalgamating multiple errors into a unified error.

## Recommendations

Make sure you address any errors propagated by the Close or CloseContext functions to effectively manage any 
complications that arise during the shutdown process.