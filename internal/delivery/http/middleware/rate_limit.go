package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"be_region_indonesia/pkg/response"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

// RedisSlidingWindowRateLimiter membatasi request per menit menggunakan Redis Sorted Set (ZSET)
func RedisSlidingWindowRateLimiter(rdb *redis.Client, limitRPM int, keyPrefix string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			clientIP := c.RealIP()
			if clientIP == "" {
				clientIP = c.Request().RemoteAddr
			}

			key := fmt.Sprintf("ratelimit:%s:%s", keyPrefix, clientIP)
			now := time.Now().UnixMilli()
			windowStart := now - (60 * 1000) // 1 menit yang lalu

			ctx := c.Request().Context()
			pipe := rdb.Pipeline()

			// 1. Bersihkan request di luar sliding window
			pipe.ZRemRangeByScore(ctx, key, "-inf", strconv.FormatInt(windowStart, 10))
			// 2. Tambahkan timestamp request saat ini
			pipe.ZAdd(ctx, key, redis.Z{Score: float64(now), Member: fmt.Sprintf("%d-%d", now, time.Now().Nanosecond())})
			// 3. Hitung jumlah request di jendela aktif
			countCmd := pipe.ZCard(ctx, key)
			// 4. Set TTL 1 menit
			pipe.Expire(ctx, key, 70*time.Second)

			_, err := pipe.Exec(ctx)
			if err != nil {
				// Fail-open: jika Redis down, jangan blokir request client
				return next(c)
			}

			requestCount := countCmd.Val()
			remaining := int64(limitRPM) - requestCount
			if remaining < 0 {
				remaining = 0
			}

			resetTime := (now / 1000) + 60
			c.Response().Header().Set("X-RateLimit-Limit", strconv.Itoa(limitRPM))
			c.Response().Header().Set("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))
			c.Response().Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetTime, 10))

			if requestCount > int64(limitRPM) {
				c.Response().Header().Set("Retry-After", "60")
				return response.Error(
					c,
					http.StatusTooManyRequests,
					"Batas kuota akses terlampaui. Silakan perlambat permintaan Anda.",
					response.FieldError{Field: "rate_limit", Issue: "Kuota permintaan per menit telah habis, silakan coba kembali dalam 1 menit."},
				)
			}

			return next(c)
		}
	}
}
