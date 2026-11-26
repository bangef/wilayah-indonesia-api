package logger

import (
	"context"
	"log/slog"
)

type contextKey string

const requestIDKey contextKey = "request_id"

// WithRequestID menyuntikkan request_id ke dalam context.Context
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// GetRequestID mengekstrak request_id dari context.Context
func GetRequestID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if reqID, ok := ctx.Value(requestIDKey).(string); ok {
		return reqID
	}
	return ""
}

// LogWithCtx mengembalikan instance *slog.Logger yang terikat dengan request_id dari context
func LogWithCtx(ctx context.Context) *slog.Logger {
	reqID := GetRequestID(ctx)
	if reqID == "" {
		return slog.Default()
	}
	return slog.Default().With(slog.String("request_id", reqID))
}
