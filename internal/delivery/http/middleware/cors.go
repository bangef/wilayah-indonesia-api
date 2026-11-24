package middleware

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
)

func SetupCORS(allowedOrigins string, allowCredentials bool, maxAgeSeconds int) echo.MiddlewareFunc {
	origins := strings.Split(allowedOrigins, ",")
	for i := range origins {
		origins[i] = strings.TrimSpace(origins[i])
	}
	// Selalu izinkan origin localhost / 127.0.0.1 untuk development dan Swagger UI
	origins = append(origins, "http://localhost:8080", "http://127.0.0.1:8080")

	return echomw.CORSWithConfig(echomw.CORSConfig{
		AllowOrigins: origins,
		AllowMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowHeaders: []string{
			echo.HeaderOrigin,
			echo.HeaderContentType,
			echo.HeaderAccept,
			echo.HeaderAuthorization,
			"X-Request-ID",
			"X-API-Key",
		},
		ExposeHeaders: []string{
			"X-RateLimit-Limit",
			"X-RateLimit-Remaining",
			"X-RateLimit-Reset",
			"X-Request-ID",
		},
		AllowCredentials: allowCredentials,
		MaxAge:           maxAgeSeconds,
	})
}
