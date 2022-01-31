package observability

import "context"

// Tracer is topology's tester tracer to interface with different implementations.
type Tracer interface {
	// StartSpanFromContext creates and starts a span.
	StartSpanFromContext(ctx context.Context, name string) Span
}
