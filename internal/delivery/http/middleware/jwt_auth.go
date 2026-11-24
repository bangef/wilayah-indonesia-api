package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"be_region_indonesia/pkg/response"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

type JWTClaims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// JWTAuthMiddleware memverifikasi Bearer Token JWT dan mengecek Redis blacklist revocation
func JWTAuthMiddleware(jwtSecret string, rdb *redis.Client, requiredRoles ...string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				return response.Error(
					c,
					http.StatusUnauthorized,
					"Header otorisasi tidak ditemukan atau format tidak valid.",
					response.FieldError{Field: "authorization", Issue: "Bearer token otentikasi wajib disertakan."},
				)
			}

			tokenStr := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))

			// Parse dan validasi algoritma penandatanganan secara eksplisit
			claims := &JWTClaims{}
			token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
				}
				return []byte(jwtSecret), nil
			})

			if err != nil || !token.Valid {
				return response.Error(
					c,
					http.StatusUnauthorized,
					"Token otentikasi tidak valid atau telah kedaluwarsa.",
					response.FieldError{Field: "token", Issue: "Token otentikasi tidak sah atau masa berlakunya telah habis."},
				)
			}

			// Cek apakah token dicabut / di-blacklist di Redis
			if claims.ID != "" && rdb != nil {
				blacklistKey := fmt.Sprintf("jwt:blacklist:%s", claims.ID)
				isBlacklisted, err := rdb.Exists(context.Background(), blacklistKey).Result()
				if err == nil && isBlacklisted > 0 {
					return response.Error(
						c,
						http.StatusUnauthorized,
						"Token otentikasi telah dicabut.",
						response.FieldError{Field: "token", Issue: "Sesi token telah dinonaktifkan di sistem."},
					)
				}
			}

			// Cek otorisasi peran (Role-Based Access Control)
			if len(requiredRoles) > 0 {
				authorized := false
				for _, r := range requiredRoles {
					if strings.EqualFold(claims.Role, r) {
						authorized = true
						break
					}
				}
				if !authorized {
					return response.Error(
						c,
						http.StatusForbidden,
						"Hak akses tidak mencukupi untuk mengakses sumber daya ini.",
						response.FieldError{Field: "role", Issue: fmt.Sprintf("Peran '%s' tidak memiliki izin untuk melakukan tindakan ini.", claims.Role)},
					)
				}
			}

			c.Set("user", claims)
			return next(c)
		}
	}
}

// GenerateTestToken helper untuk testing otentikasi JWT
func GenerateTestToken(jwtSecret, userID, role string) (string, error) {
	if strings.TrimSpace(jwtSecret) == "" {
		return "", errors.New("jwtSecret cannot be empty")
	}
	claims := JWTClaims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			ID:        "test-token-id-12345",
			Issuer:    "be-region-auth-service",
			Audience:  jwt.ClaimStrings{"be-region-api"},
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(jwtSecret))
}
