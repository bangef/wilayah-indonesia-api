package middleware_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"be_region_indonesia/internal/delivery/http/middleware"
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSlogMiddleware(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	l := slog.New(handler).With(
		slog.String("service", "be_region_indonesia"),
		slog.String("env", "test"),
	)
	slog.SetDefault(l)

	e := echo.New()
	e.Use(echomw.Recover())
	e.Use(echomw.RequestID())
	e.Use(middleware.SlogMiddleware())

	// 1. Route 200 OK
	e.GET("/api/v1/provinces", func(c echo.Context) error {
		return c.String(http.StatusOK, "success")
	})

	// 2. Route 400 Bad Request
	e.GET("/api/v1/bad", func(c echo.Context) error {
		return c.String(http.StatusBadRequest, "bad request")
	})

	// 3. Route 500 Internal Error
	e.GET("/api/v1/error", func(c echo.Context) error {
		return errors.New("database connection lost")
	})

	// Test 1: 200 OK -> Level INFO
	{
		buf.Reset()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/provinces?limit=10", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var entry map[string]interface{}
		err := json.Unmarshal(buf.Bytes(), &entry)
		require.NoError(t, err)

		assert.Equal(t, "INFO", entry["level"])
		assert.Equal(t, "http request completed", entry["msg"])
		assert.Equal(t, "GET", entry["method"])
		assert.Equal(t, "/api/v1/provinces", entry["path"])
		assert.Equal(t, "/api/v1/provinces?limit=10", entry["uri"])
		assert.Equal(t, float64(200), entry["status"])
		assert.NotEmpty(t, entry["request_id"])
		assert.NotEmpty(t, entry["latency_ms"])
		assert.NotEmpty(t, entry["remote_ip"])
	}

	// Test 2: 400 Bad Request -> Level WARN
	{
		buf.Reset()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/bad", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		var entry map[string]interface{}
		err := json.Unmarshal(buf.Bytes(), &entry)
		require.NoError(t, err)

		assert.Equal(t, "WARN", entry["level"])
		assert.Equal(t, "http client error", entry["msg"])
		assert.Equal(t, float64(400), entry["status"])
	}

	// Test 3: 500 Internal Error -> Level ERROR
	{
		buf.Reset()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/error", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		var entry map[string]interface{}
		err := json.Unmarshal(buf.Bytes(), &entry)
		require.NoError(t, err)

		assert.Equal(t, "ERROR", entry["level"])
		assert.Equal(t, "http request failed", entry["msg"])
		assert.Equal(t, float64(500), entry["status"])
		assert.Contains(t, entry["error"], "database connection lost")
	}
}
