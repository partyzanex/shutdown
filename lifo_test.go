package shutdown

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLifoCloseContext(t *testing.T) {
	lifo := &Lifo{}

	first := &mockCloser{closeFunc: func() error { return nil }}
	second := &mockCloser{closeFunc: func() error { return errors.New("second error") }}
	third := &mockCloser{closeFunc: func() error { return nil }}

	lifo.Append(first)
	lifo.Append(second)
	lifo.Append(third)

	err := lifo.CloseContext(context.Background())
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "second error")

	lifo = &Lifo{}
	timeoutCloser := &mockCloser{closeFunc: func() error {
		time.Sleep(2 * time.Second)
		return nil
	}}

	lifo.Append(timeoutCloser)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err = lifo.CloseContext(ctx)
	assert.NotNil(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded))
}

func TestLifoClose(t *testing.T) {
	lifo := &Lifo{}

	first := &mockCloser{closeFunc: func() error { return nil }}
	second := &mockCloser{closeFunc: func() error { return errors.New("second error") }}

	lifo.Append(first)
	lifo.Append(second)

	err := lifo.Close()
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "second error")
}
