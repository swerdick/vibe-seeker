package observability

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestLogger_NoSpan(t *testing.T) {
	logger := Logger(context.Background())
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestLogger_WithSpan(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer func() { _ = tp.Shutdown(context.Background()) }()

	ctx, span := tp.Tracer("test").Start(context.Background(), "test-span")
	defer span.End()

	// Capture log output.
	var buf bytes.Buffer
	origLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	defer slog.SetDefault(origLogger)

	logger := Logger(ctx)
	logger.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "trace_id") {
		t.Errorf("expected trace_id in log output, got: %s", output)
	}
	if !strings.Contains(output, "span_id") {
		t.Errorf("expected span_id in log output, got: %s", output)
	}
}
