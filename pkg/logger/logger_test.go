package logger_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"be_region_indonesia/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitLogger_AndContextPropagation(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	l := slog.New(handler).With(
		slog.String("service", "be_region_indonesia"),
		slog.String("env", "test"),
	)
	slog.SetDefault(l)

	ctx := context.Background()
	reqID := "test-req-12345"

	// 1. Without Request ID
	logger.LogWithCtx(ctx).Info("pesan tanpa request id", slog.String("action", "test_action"))

	var logData1 map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logData1)
	require.NoError(t, err)

	assert.Equal(t, "be_region_indonesia", logData1["service"])
	assert.Equal(t, "test", logData1["env"])
	assert.Equal(t, "pesan tanpa request id", logData1["msg"])
	assert.Equal(t, "test_action", logData1["action"])
	assert.Nil(t, logData1["request_id"])

	buf.Reset()

	// 2. With Request ID
	ctxWithReq := logger.WithRequestID(ctx, reqID)
	assert.Equal(t, reqID, logger.GetRequestID(ctxWithReq))

	logger.LogWithCtx(ctxWithReq).Warn("pesan dengan request id", slog.Int("count", 42))

	var logData2 map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &logData2)
	require.NoError(t, err)

	assert.Equal(t, "be_region_indonesia", logData2["service"])
	assert.Equal(t, "WARN", logData2["level"])
	assert.Equal(t, "pesan dengan request id", logData2["msg"])
	assert.Equal(t, reqID, logData2["request_id"])
	assert.Equal(t, float64(42), logData2["count"])
}
