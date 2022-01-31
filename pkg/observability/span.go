package observability

import "context"

// Span interface as returned by Tracer.StarSpan
type Span interface {
	// Context returns the Span's SpanContext.
	Context() context.Context
	// TraceID returns the Span's trace identifier.
	TraceID() string
	// SetName updates the Span's name.
	SetName(string)
	// Tag sets Tag with given key and value to the Span. If key already exists in
	// the Span the value will be overridden except for error tags where the first
	// value is persisted.
	Tag(string, string)
	// Finish the Span and send to Reporter.
	Finish()
}
