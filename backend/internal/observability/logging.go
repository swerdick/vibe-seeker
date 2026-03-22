package observability

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

// Logger returns an slog.Logger enriched with trace_id and span_id from the
// given context. If the context has no active span, the default logger is
// returned without extra attributes.
func Logger(ctx context.Context) *slog.Logger {
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		return slog.Default()
	}
	return slog.Default().With(
		slog.String("trace_id", span.SpanContext().TraceID().String()),
		slog.String("span_id", span.SpanContext().SpanID().String()),
	)
}
