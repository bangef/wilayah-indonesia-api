package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"be_region_indonesia/pkg/response"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

type JWTClaims struct {
	UserID      string   `json:"user_id"`
	Username    string   `json:"username"`
	RegionScope string   `json:"region_scope"`
	Roles       []string `json:"roles"`
	Role        string   `json:"role,omitempty"` // legacy compatibility
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

			// Cek otorisasi peran jika parameter requiredRoles diberikan
			if len(requiredRoles) > 0 {
				hasRole := false

				// Check legacy claims.Role
				if claims.Role != "" {
					for _, req := range requiredRoles {
						if strings.EqualFold(claims.Role, req) {
							hasRole = true
							break
						}
					}
				}

				// Check roles array
				if !hasRole {
					for _, r := range claims.Roles {
						for _, req := range requiredRoles {
							if strings.EqualFold(r, req) {
								hasRole = true
								break
							}
						}
						if hasRole {
							break
						}
					}
				}

				if !hasRole {
					return response.Error(
						c,
						http.StatusForbidden,
						"Akses ditolak. Peran Anda tidak memiliki hak untuk mengakses endpoint ini.",
						response.FieldError{Field: "role", Issue: fmt.Sprintf("Dibutuhkan salah satu dari peran: %v", requiredRoles)},
					)
				}
			}

			// Set claims dan raw token ke konteks
			c.Set("user", claims)
			c.Set("token", token)
			c.Set("raw_token", tokenStr)

			return next(c)
		}
	}
}

// GenerateTestToken helper untuk testing
func GenerateTestToken(jwtSecret, userID, role string) (string, error) {
	claims := &JWTClaims{
		UserID:   userID,
		Username: "admin",
		Roles:    []string{role},
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: userID,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(jwtSecret))
}
