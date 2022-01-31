package observability

import "golang.org/x/net/context"

// Contexter is a extension interface to retrieve current span from Go's context.
type Contexter interface {
	// SpanFromContext retrieves a Span from Go's context propagation
	// mechanism if found. If not found, returns nil.
	SpanFromContext(ctx context.Context) Span
}
