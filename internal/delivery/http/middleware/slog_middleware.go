package middleware

import (
	"log/slog"
	"time"

	"be_region_indonesia/pkg/logger"
	"github.com/labstack/echo/v4"
)

// SlogMiddleware menghasilkan structured JSON log untuk setiap siklus HTTP request
// yang dipancarkan ke os.Stdout agar ditangkap oleh systemd-journald dan Vector.
func SlogMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			// 1. Ambil request_id dari response header atau request header
			reqID := c.Response().Header().Get(echo.HeaderXRequestID)
			if reqID == "" {
				reqID = c.Request().Header.Get(echo.HeaderXRequestID)
			}

			// 2. Suntikkan request_id ke dalam context.Context request
			ctx := logger.WithRequestID(c.Request().Context(), reqID)
			c.SetRequest(c.Request().WithContext(ctx))

			// 3. Jalankan rantai handler berikutnya
			err := next(c)
			if err != nil {
				c.Error(err)
			}

			// 4. Hitung latensi dalam milidetik (float64 numerik untuk kemudahan kueri di LogsQL)
			latency := time.Since(start)
			latencyMs := float64(latency.Microseconds()) / 1000.0

			req := c.Request()
			res := c.Response()

			// 5. Susun atribut structured log
			attrs := []any{
				slog.String("method", req.Method),
				slog.String("path", c.Path()), // Route pattern (e.g., /api/v1/provinces)
				slog.String("uri", req.RequestURI),
				slog.Int("status", res.Status),
				slog.Float64("latency_ms", latencyMs),
				slog.String("remote_ip", c.RealIP()),
			}

			if reqID != "" {
				attrs = append(attrs, slog.String("request_id", reqID))
			}

			if err != nil {
				attrs = append(attrs, slog.String("error", err.Error()))
			}

			// 6. Klasifikasi Log Level berdasarkan HTTP Status
			switch {
			case res.Status >= 500:
				slog.Error("http request failed", attrs...)
			case res.Status >= 400:
				slog.Warn("http client error", attrs...)
			default:
				slog.Info("http request completed", attrs...)
			}

			return nil
		}
	}
}
