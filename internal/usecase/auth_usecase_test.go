package usecase_test

import (
	"context"
	"testing"

	"be_region_indonesia/config"
	"be_region_indonesia/internal/domain"
	"be_region_indonesia/internal/usecase"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthUsecase_Login(t *testing.T) {
	cfg := &config.Config{
		JWTSecret:            "super-secret-jwt-key-minimum-32-bytes-long",
		JWTExpirationM:       15,
		JWTIssuer:            "test-issuer",
		JWTAudience:          "test-audience",
		AdminDefaultUsername: "admin",
		AdminDefaultPassword: "AdminRegional2026!",
	}

	uc := usecase.NewAuthUsecase(cfg)
	ctx := context.Background()

	t.Run("Login_Success", func(t *testing.T) {
		res, err := uc.Login(ctx, "admin", "AdminRegional2026!")
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, "Bearer", res.TokenType)
		assert.Equal(t, 900, res.ExpiresInSeconds)
		assert.Equal(t, "admin", res.User.Username)
		assert.Equal(t, "ADMIN", res.User.Role)
		assert.NotEmpty(t, res.AccessToken)

		// Parse token to verify claims
		token, err := jwt.Parse(res.AccessToken, func(token *jwt.Token) (interface{}, error) {
			return []byte(cfg.JWTSecret), nil
		})
		require.NoError(t, err)
		assert.True(t, token.Valid)

		claims, ok := token.Claims.(jwt.MapClaims)
		require.True(t, ok)
		assert.Equal(t, "usr_admin_001", claims["user_id"])
		assert.Equal(t, "ADMIN", claims["role"])
		assert.Equal(t, "test-issuer", claims["iss"])
	})

	t.Run("Login_InvalidUsername", func(t *testing.T) {
		res, err := uc.Login(ctx, "wronguser", "AdminRegional2026!")
		assert.ErrorIs(t, err, domain.ErrInvalidCredentials)
		assert.Nil(t, res)
	})

	t.Run("Login_InvalidPassword", func(t *testing.T) {
		res, err := uc.Login(ctx, "admin", "wrongpassword")
		assert.ErrorIs(t, err, domain.ErrInvalidCredentials)
		assert.Nil(t, res)
	})

	t.Run("Login_EmptyCredentials", func(t *testing.T) {
		res, err := uc.Login(ctx, "", "")
		assert.ErrorIs(t, err, domain.ErrInvalidCredentials)
		assert.Nil(t, res)
	})
}
