// Package e2botel provides OpenTelemetry integration for the E2B Go SDK.
//
// It extracts W3C Trace Context (traceparent + tracestate) from a
// context.Context carrying an OTel span and converts them into E2B
// sandbox options, enabling distributed tracing propagation into
// sandbox environments.
package e2botel

import (
	"context"
	"fmt"

	e2b "github.com/xerpa-ai/e2b-go"
	"go.opentelemetry.io/otel/trace"
)

// WithTraceContext extracts the W3C Trace Context from ctx and returns
// E2B options that inject TRACEPARENT (and TRACESTATE when present) as
// sandbox environment variables.
//
// Returns nil if ctx does not carry a valid span context.
//
// Usage:
//
//	opts := []e2b.Option{e2b.WithTemplate("..."), e2b.WithTimeout(5*time.Minute)}
//	opts = append(opts, e2botel.WithTraceContext(ctx)...)
//	sandbox, err := e2b.NewWithContext(ctx, opts...)
func WithTraceContext(ctx context.Context) []e2b.Option {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return nil
	}

	// W3C Trace Context spec-defined flag bits: sampled (0x01) and random (0x02).
	flags := sc.TraceFlags() & 0x03
	tp := fmt.Sprintf("00-%s-%s-%02x", sc.TraceID(), sc.SpanID(), byte(flags))

	opts := []e2b.Option{e2b.WithTraceparent(tp)}

	if ts := sc.TraceState().String(); ts != "" {
		opts = append(opts, e2b.WithTracestate(ts))
	}

	return opts
}
