package logger

import (
	"context"

	"go.uber.org/zap"
)

type ctxKey string

const (
	// TraceIDKey context 中 trace_id 的 key
	TraceIDKey ctxKey = "trace_id"
	// UserOpenIDKey context 中用户 openid 的 key
	UserOpenIDKey ctxKey = "user_open_id"
)

// ContextWithTraceID 将 trace_id 写入 context
func ContextWithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, TraceIDKey, traceID)
}

// TraceIDFromContext 从 context 提取 trace_id
func TraceIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(TraceIDKey).(string)
	return v, ok && v != ""
}

// ContextWithUserOpenID 将 open_id 写入 context
func ContextWithUserOpenID(ctx context.Context, openID string) context.Context {
	return context.WithValue(ctx, UserOpenIDKey, openID)
}

// OpenIDFromContext 从 context 提取 open_id
func OpenIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(UserOpenIDKey).(string)
	return v, ok && v != ""
}

// WithTraceID 返回携带 trace_id 字段的 logger
func WithTraceID(ctx context.Context) *zap.Logger {
	if traceID, ok := TraceIDFromContext(ctx); ok {
		return L().With(zap.String("trace_id", traceID))
	}
	return L()
}

// WithContext 返回携带 trace_id + open_id 字段的 logger
func WithContext(ctx context.Context) *zap.Logger {
	fields := make([]zap.Field, 0, 2)
	if traceID, ok := TraceIDFromContext(ctx); ok {
		fields = append(fields, zap.String("trace_id", traceID))
	}
	if openID, ok := OpenIDFromContext(ctx); ok {
		fields = append(fields, zap.String("open_id", openID))
	}
	if len(fields) == 0 {
		return L()
	}
	return L().With(fields...)
}
