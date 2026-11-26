package usecase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"

	"be_region_indonesia/config"
	"be_region_indonesia/internal/delivery/http/middleware"
	"be_region_indonesia/internal/domain"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type authUsecase struct {
	cfg       *config.Config
	authRepo  domain.AuthRepository
	authCache domain.AuthCacheRepository
}

func NewAuthUsecase(cfg *config.Config, authRepo domain.AuthRepository, authCache domain.AuthCacheRepository) domain.AuthUsecase {
	return &authUsecase{
		cfg:       cfg,
		authRepo:  authRepo,
		authCache: authCache,
	}
}

func hashSHA256(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func generateRandomHex(byteCount int) (string, error) {
	bytes := make([]byte, byteCount)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func (u *authUsecase) Login(ctx context.Context, username, password string) (*domain.LoginResult, error) {
	user, passwordHash, err := u.authRepo.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, domain.ErrInvalidCredentials
	}

	if !user.IsActive {
		return nil, domain.ErrUserDisabled
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
		return nil, domain.ErrInvalidCredentials
	}

	familyID := uuid.New().String()
	tokenPair, err := u.generateTokenPairWithFamily(ctx, user, familyID)
	if err != nil {
		return nil, err
	}

	// Warm-up Redis permissions cache
	perms, err := u.authRepo.GetUserPermissions(ctx, user.ID)
	if err == nil && len(perms) > 0 {
		_ = u.authCache.SetUserPermissions(ctx, user.ID, perms, 1*time.Hour)
	}

	return &domain.LoginResult{
		AccessToken:      tokenPair.AccessToken,
		RefreshToken:     tokenPair.RefreshToken,
		TokenType:        tokenPair.TokenType,
		ExpiresInSeconds: tokenPair.ExpiresInSeconds,
		User:             *user,
	}, nil
}

func (u *authUsecase) generateTokenPairWithFamily(ctx context.Context, user *domain.User, familyID string) (*domain.TokenPair, error) {
	// 1. Generate JWT Access Token
	jti, err := generateRandomHex(16)
	if err != nil {
		return nil, fmt.Errorf("failed to generate jti: %w", err)
	}

	accessTTL := time.Duration(u.cfg.JWTExpirationM) * time.Minute
	if accessTTL <= 0 {
		accessTTL = 15 * time.Minute
	}
	accessExpiresAt := time.Now().Add(accessTTL)

	var roleCodes []string
	for _, r := range user.Roles {
		roleCodes = append(roleCodes, r.Code)
	}

	claims := &middleware.JWTClaims{
		UserID:      user.ID,
		Username:    user.Username,
		RegionScope: user.RegionScope,
		Roles:       roleCodes,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			Subject:   user.ID,
			Issuer:    u.cfg.AppName,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(accessExpiresAt),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedAccessToken, err := token.SignedString([]byte(u.cfg.JWTSecret))
	if err != nil {
		return nil, fmt.Errorf("failed to sign access token: %w", err)
	}

	// 2. Generate Opaque Refresh Token
	rawRefreshToken, err := generateRandomHex(32) // 64 chars hex
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}
	tokenHash := hashSHA256(rawRefreshToken)

	refreshTTL := 7 * 24 * time.Hour
	refreshExpiresAt := time.Now().Add(refreshTTL)

	storedToken := &domain.RefreshToken{
		ID:        uuid.New().String(),
		UserID:    user.ID,
		TokenHash: tokenHash,
		FamilyID:  familyID,
		IsRevoked: false,
		ExpiresAt: refreshExpiresAt,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := u.authRepo.SaveRefreshToken(ctx, storedToken); err != nil {
		return nil, fmt.Errorf("failed to store refresh token: %w", err)
	}

	return &domain.TokenPair{
		AccessToken:      signedAccessToken,
		RefreshToken:     rawRefreshToken,
		TokenType:        "Bearer",
		ExpiresInSeconds: int(accessTTL.Seconds()),
	}, nil
}

func (u *authUsecase) RefreshToken(ctx context.Context, rawRefreshToken string) (*domain.TokenPair, error) {
	tokenHash := hashSHA256(rawRefreshToken)
	storedToken, err := u.authRepo.GetRefreshTokenByHash(ctx, tokenHash)
	if err != nil || storedToken == nil {
		return nil, domain.ErrInvalidRefreshToken
	}

	// 1. Token Reuse Detection
	if storedToken.IsRevoked {
		_ = u.authRepo.RevokeTokenFamily(ctx, storedToken.FamilyID)
		slog.Warn("Potensi pencurian token terdeteksi! Membatalkan seluruh family token",
			slog.String("user_id", storedToken.UserID),
			slog.String("family_id", storedToken.FamilyID),
		)
		return nil, domain.ErrTokenTheftDetected
	}

	// 2. Expiration check
	if time.Now().After(storedToken.ExpiresAt) {
		return nil, domain.ErrRefreshTokenExpired
	}

	// 3. Mark old token as revoked (Rotate)
	if err := u.authRepo.RevokeRefreshToken(ctx, tokenHash); err != nil {
		return nil, err
	}

	// 4. Retrieve user and issue new pair within same family
	user, err := u.authRepo.GetUserByID(ctx, storedToken.UserID)
	if err != nil || !user.IsActive {
		return nil, domain.ErrUserDisabled
	}

	return u.generateTokenPairWithFamily(ctx, user, storedToken.FamilyID)
}

func (u *authUsecase) Logout(ctx context.Context, accessTokenJTI string, accessTokenTTL time.Duration, rawRefreshToken string, userID string) error {
	// 1. Blacklist Access Token JTI
	if accessTokenJTI != "" && accessTokenTTL > 0 {
		_ = u.authCache.BlacklistAccessToken(ctx, accessTokenJTI, accessTokenTTL)
	}

	// 2. Revoke Refresh Token or all tokens for user
	if rawRefreshToken != "" {
		tokenHash := hashSHA256(rawRefreshToken)
		_ = u.authRepo.RevokeRefreshToken(ctx, tokenHash)
	} else if userID != "" {
		_ = u.authRepo.RevokeAllUserRefreshTokens(ctx, userID)
	}

	// 3. Invalidate user permissions in cache
	if userID != "" {
		_ = u.authCache.InvalidateUserPermissions(ctx, userID)
	}

	return nil
}

func (u *authUsecase) GetMe(ctx context.Context, userID string) (*domain.User, []string, error) {
	user, err := u.authRepo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	perms, err := u.authRepo.GetUserPermissions(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	return user, perms, nil
}
