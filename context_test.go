package shutdown

import (
	"context"
	"testing"
)

func TestClosureToContext(t *testing.T) {
	closure := &Lifo{}
	ctx := ClosureToContext(context.Background(), closure)

	extractedClosure, ok := ClosureFromContext(ctx)
	if !ok || extractedClosure != closure {
		t.Fatalf("Expected to retrieve the original closure from context, but got %v", extractedClosure)
	}
}

func TestClosureFromContext_NoClosure(t *testing.T) {
	ctx := context.Background()

	extractedClosure, ok := ClosureFromContext(ctx)
	if ok || extractedClosure != nil {
		t.Fatalf("Expected no closure in context, but got %v", extractedClosure)
	}
}
