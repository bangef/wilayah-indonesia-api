package middleware

import (
	"strings"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
)

func SetupSecureHeaders() echo.MiddlewareFunc {
	return echomw.SecureWithConfig(echomw.SecureConfig{
		Skipper: func(c echo.Context) bool {
			// Skip atau sesuaikan header untuk rute Swagger UI
			return strings.HasPrefix(c.Path(), "/swagger")
		},
		XSSProtection:         "1; mode=block",
		ContentTypeNosniff:    "nosniff",
		XFrameOptions:         "DENY",
		HSTSMaxAge:            31536000,
		HSTSExcludeSubdomains: false,
		HSTSPreloadEnabled:    true,
		ContentSecurityPolicy: "default-src 'none'; frame-ancestors 'none';",
	})
}

