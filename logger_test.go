package logger

import (
	"context"
	"testing"
)

func TestContextRoundTrip(t *testing.T) {
	ctx := context.Background()
	ctx = ContextWithTraceID(ctx, "abc123")
	ctx = ContextWithUserOpenID(ctx, "oPenID")

	if v, ok := TraceIDFromContext(ctx); !ok || v != "abc123" {
		t.Fatalf("trace_id mismatch: got %q ok=%v", v, ok)
	}
	if v, ok := OpenIDFromContext(ctx); !ok || v != "oPenID" {
		t.Fatalf("open_id mismatch: got %q ok=%v", v, ok)
	}
}

func TestEmptyContext(t *testing.T) {
	ctx := context.Background()
	if _, ok := TraceIDFromContext(ctx); ok {
		t.Fatal("expected no trace_id on empty ctx")
	}
	if _, ok := OpenIDFromContext(ctx); ok {
		t.Fatal("expected no open_id on empty ctx")
	}
	// WithContext 应返回全局 logger 本身，不 panic
	_ = WithContext(ctx)
	_ = WithTraceID(ctx)
}

func TestParseLevelFallback(t *testing.T) {
	if parseLevel("nonsense").String() != "info" {
		t.Fatal("unknown level should fall back to info")
	}
}
