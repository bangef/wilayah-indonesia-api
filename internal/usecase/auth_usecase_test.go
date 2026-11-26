package usecase_test

import (
	"context"
	"path/filepath"
	"testing"

	"be_region_indonesia/config"
	"be_region_indonesia/internal/domain"
	"be_region_indonesia/internal/repository/cache"
	"be_region_indonesia/internal/repository/postgres"
	"be_region_indonesia/internal/usecase"
	"be_region_indonesia/pkg/database"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAuthTest(t *testing.T) (*config.Config, domain.AuthUsecase, domain.AuthRepository, domain.AuthCacheRepository, func()) {
	rootEnv := filepath.Join("..", "..", ".env")
	cfg, err := config.LoadConfig(rootEnv)
	require.NoError(t, err)

	pgPool, err := database.NewPostgresConnection(cfg)
	require.NoError(t, err)

	redisPool, err := database.NewRedisConnection(cfg)
	require.NoError(t, err)

	authRepo := postgres.NewAuthRepository(pgPool.DB)
	authCache := cache.NewRedisAuthCache(redisPool.Client)
	authUc := usecase.NewAuthUsecase(cfg, authRepo, authCache)

	cleanup := func() {
		pgPool.Close()
		redisPool.Close()
	}

	return cfg, authUc, authRepo, authCache, cleanup
}

func TestAuthUsecase_Login(t *testing.T) {
	cfg, uc, _, _, cleanup := setupAuthTest(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("Login_Success", func(t *testing.T) {
		res, err := uc.Login(ctx, "admin", "AdminRegional2026!")
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, "Bearer", res.TokenType)
		assert.Equal(t, 900, res.ExpiresInSeconds)
		assert.Equal(t, "admin", res.User.Username)
		assert.NotEmpty(t, res.AccessToken)
		assert.NotEmpty(t, res.RefreshToken)

		// Parse token to verify claims
		token, err := jwt.Parse(res.AccessToken, func(token *jwt.Token) (interface{}, error) {
			return []byte(cfg.JWTSecret), nil
		})
		require.NoError(t, err)
		assert.True(t, token.Valid)

		claims, ok := token.Claims.(jwt.MapClaims)
		require.True(t, ok)
		assert.Equal(t, "usr_root_superadmin", claims["user_id"])
		assert.Equal(t, "admin", claims["username"])
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

func TestAuthUsecase_RefreshTokenRotation_And_ReuseDetection(t *testing.T) {
	_, uc, _, _, cleanup := setupAuthTest(t)
	defer cleanup()

	ctx := context.Background()

	// 1. Login to get initial pair
	loginRes, err := uc.Login(ctx, "admin", "AdminRegional2026!")
	require.NoError(t, err)
	require.NotEmpty(t, loginRes.RefreshToken)

	initialRefreshToken := loginRes.RefreshToken

	// 2. Rotate token
	pair1, err := uc.RefreshToken(ctx, initialRefreshToken)
	require.NoError(t, err)
	require.NotEmpty(t, pair1.AccessToken)
	require.NotEmpty(t, pair1.RefreshToken)
	assert.NotEqual(t, initialRefreshToken, pair1.RefreshToken)

	// 3. Second rotation using new token
	pair2, err := uc.RefreshToken(ctx, pair1.RefreshToken)
	require.NoError(t, err)
	require.NotEmpty(t, pair2.AccessToken)
	require.NotEmpty(t, pair2.RefreshToken)
	assert.NotEqual(t, pair1.RefreshToken, pair2.RefreshToken)

	// 4. Token Reuse Detection: Submit initialRefreshToken (already rotated)
	theftRes, err := uc.RefreshToken(ctx, initialRefreshToken)
	assert.ErrorIs(t, err, domain.ErrTokenTheftDetected)
	assert.Nil(t, theftRes)

	// 5. Check that whole family is revoked: pair2 should now fail
	revokedRes, err := uc.RefreshToken(ctx, pair2.RefreshToken)
	assert.ErrorIs(t, err, domain.ErrTokenTheftDetected)
	assert.Nil(t, revokedRes)
}

func TestValidateJurisdiction(t *testing.T) {
	// Superadmin / National scope
	assert.True(t, usecase.ValidateJurisdiction("*", "32.04.05.2001"))
	assert.True(t, usecase.ValidateJurisdiction("", "35.78"))

	// Provincial scope (32 = Jabar)
	assert.True(t, usecase.ValidateJurisdiction("32", "32"))
	assert.True(t, usecase.ValidateJurisdiction("32", "32.04"))
	assert.True(t, usecase.ValidateJurisdiction("32", "32.04.05.2001"))
	assert.False(t, usecase.ValidateJurisdiction("32", "35.78.01.1001")) // Jatim
	assert.False(t, usecase.ValidateJurisdiction("32", "320"))          // not dot prefix
}
