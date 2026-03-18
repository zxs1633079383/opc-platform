package trace

import (
	"context"
	"testing"
)

func TestInitTracer_NoopWhenDisabled(t *testing.T) {
	shutdown, err := InitTracer(Config{Enabled: false})
	if err != nil {
		t.Fatalf("InitTracer with disabled config returned error: %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected non-nil shutdown function")
	}

	tracer := Tracer()
	if tracer == nil {
		t.Fatal("expected non-nil tracer even when disabled")
	}

	// Shutdown should succeed without error
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown returned error: %v", err)
	}
}

func TestStartSpan(t *testing.T) {
	// Initialize with noop tracer for testing
	shutdown, err := InitTracer(Config{Enabled: false})
	if err != nil {
		t.Fatalf("InitTracer returned error: %v", err)
	}
	defer func() {
		if err := shutdown(context.Background()); err != nil {
			t.Errorf("shutdown returned error: %v", err)
		}
	}()

	ctx, span := StartSpan(context.Background(), "test-span")
	if ctx == nil {
		t.Fatal("expected non-nil context from StartSpan")
	}
	if span == nil {
		t.Fatal("expected non-nil span from StartSpan")
	}

	// span.End() should not panic
	span.End()
}

func TestSpanIDFromContext_Empty(t *testing.T) {
	// With a bare context (no span), SpanIDFromContext should return ""
	id := SpanIDFromContext(context.Background())
	if id != "" {
		t.Fatalf("expected empty span ID, got %q", id)
	}
}

func TestTraceIDFromContext_Empty(t *testing.T) {
	// With a bare context (no span), TraceIDFromContext should return ""
	id := TraceIDFromContext(context.Background())
	if id != "" {
		t.Fatalf("expected empty trace ID, got %q", id)
	}
}
