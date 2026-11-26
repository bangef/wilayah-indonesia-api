package middleware

import (
	"fmt"
	"net/http"
	"time"

	"be_region_indonesia/internal/domain"
	"be_region_indonesia/pkg/response"

	"github.com/labstack/echo/v4"
)

// RequirePermission memverifikasi apakah pengguna terotentikasi memiliki hak akses yang dibutuhkan
func RequirePermission(
	authRepo domain.AuthRepository,
	cacheRepo domain.AuthCacheRepository,
	requiredPermission string,
) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			claims, ok := c.Get("user").(*JWTClaims)
			if !ok || claims == nil || claims.UserID == "" {
				return response.Error(c, http.StatusUnauthorized, "Sesi otentikasi tidak valid.")
			}

			// 1. Bypass otomatis untuk Superadmin
			if claims.Role == "SUPERADMIN" || claims.Role == "ADMIN" {
				return next(c)
			}
			for _, r := range claims.Roles {
				if r == "SUPERADMIN" || r == "ADMIN" {
					return next(c)
				}
			}

			// 2. Cek Cache Redis Set (auth:perms:<user_id>)
			ctx := c.Request().Context()
			if cacheRepo != nil {
				cachedPerms, hit, err := cacheRepo.GetUserPermissions(ctx, claims.UserID)
				if err == nil && hit {
					for _, p := range cachedPerms {
						if p == requiredPermission || p == "*:*" {
							return next(c)
						}
					}
					return response.Error(
						c,
						http.StatusForbidden,
						"Akses ditolak. Anda tidak memiliki izin untuk melakukan tindakan ini.",
						response.FieldError{
							Field: "permission",
							Issue: fmt.Sprintf("Dibutuhkan izin '%s'.", requiredPermission),
						},
					)
				}
			}

			// 3. Fallback: Query Database PostgreSQL
			if authRepo != nil {
				perms, err := authRepo.GetUserPermissions(ctx, claims.UserID)
				if err != nil {
					return response.Error(c, http.StatusInternalServerError, "Gagal memverifikasi izin pengguna.")
				}

				// Simpan ke Redis Set dengan TTL 1 Jam
				if cacheRepo != nil {
					_ = cacheRepo.SetUserPermissions(ctx, claims.UserID, perms, 1*time.Hour)
				}

				// 4. Evaluasi Izin
				for _, p := range perms {
					if p == requiredPermission || p == "*:*" {
						return next(c)
					}
				}
			}

			return response.Error(
				c,
				http.StatusForbidden,
				"Akses ditolak. Anda tidak memiliki izin untuk melakukan tindakan ini.",
				response.FieldError{
					Field: "permission",
					Issue: fmt.Sprintf("Dibutuhkan izin '%s'.", requiredPermission),
				},
			)
		}
	}
}
