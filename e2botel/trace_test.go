package e2botel

import (
	"context"
	"regexp"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

var traceparentRe = regexp.MustCompile(`^00-[0-9a-f]{32}-[0-9a-f]{16}-[0-9a-f]{2}$`)

func TestWithTraceContext_ValidSpan(t *testing.T) {
	traceID, _ := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	spanID, _ := trace.SpanIDFromHex("00f067aa0ba902b7")
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
		Remote:     true,
	})
	ctx := trace.ContextWithRemoteSpanContext(context.Background(), sc)

	opts := WithTraceContext(ctx)
	if len(opts) == 0 {
		t.Fatal("expected at least one option for valid span context")
	}
}

func TestWithTraceContext_WithTraceState(t *testing.T) {
	traceID, _ := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	spanID, _ := trace.SpanIDFromHex("00f067aa0ba902b7")
	ts, _ := trace.ParseTraceState("congo=t61rcWkgMzE,rojo=00f067aa0ba902b7")
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
		TraceState: ts,
		Remote:     true,
	})
	ctx := trace.ContextWithRemoteSpanContext(context.Background(), sc)

	opts := WithTraceContext(ctx)
	if len(opts) != 2 {
		t.Fatalf("expected 2 options (traceparent + tracestate), got %d", len(opts))
	}
}

func TestWithTraceContext_NoSpan(t *testing.T) {
	opts := WithTraceContext(context.Background())
	if opts != nil {
		t.Fatalf("expected nil for context without span, got %d options", len(opts))
	}
}

func TestWithTraceContext_InvalidSpan(t *testing.T) {
	sc := trace.NewSpanContext(trace.SpanContextConfig{})
	ctx := trace.ContextWithRemoteSpanContext(context.Background(), sc)

	opts := WithTraceContext(ctx)
	if opts != nil {
		t.Fatalf("expected nil for invalid span context, got %d options", len(opts))
	}
}
