package usecase

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"be_region_indonesia/config"
	"be_region_indonesia/internal/delivery/http/middleware"
	"be_region_indonesia/internal/domain"

	"github.com/golang-jwt/jwt/v5"
)

type authUsecase struct {
	cfg *config.Config
}

func NewAuthUsecase(cfg *config.Config) domain.AuthUsecase {
	return &authUsecase{
		cfg: cfg,
	}
}

func (u *authUsecase) Login(ctx context.Context, username, password string) (*domain.LoginResult, error) {
	expectedUsername := u.cfg.AdminDefaultUsername
	if expectedUsername == "" {
		expectedUsername = "admin"
	}

	expectedPassword := u.cfg.AdminDefaultPassword
	if expectedPassword == "" {
		expectedPassword = "AdminRegional2026!"
	}

	// Constant-time comparison to prevent timing attacks on username and password
	userMatch := subtle.ConstantTimeCompare([]byte(username), []byte(expectedUsername)) == 1
	passMatch := subtle.ConstantTimeCompare([]byte(password), []byte(expectedPassword)) == 1

	if !userMatch || !passMatch {
		return nil, domain.ErrInvalidCredentials
	}

	// Generate unique JTI (token id) for revocation capability
	tokenBytes := make([]byte, 16)
	_, _ = rand.Read(tokenBytes)
	jti := hex.EncodeToString(tokenBytes)

	userID := "usr_admin_001"
	role := "ADMIN"
	expiration := time.Duration(u.cfg.JWTExpirationM) * time.Minute
	if expiration <= 0 {
		expiration = 15 * time.Minute
	}
	expiresAt := time.Now().Add(expiration)

	issuer := u.cfg.JWTIssuer
	if strings.TrimSpace(issuer) == "" {
		issuer = "be-region-auth-service"
	}

	audience := u.cfg.JWTAudience
	if strings.TrimSpace(audience) == "" {
		audience = "be-region-api"
	}

	claims := middleware.JWTClaims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			ID:        jti,
			Issuer:    issuer,
			Audience:  jwt.ClaimStrings{audience},
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(u.cfg.JWTSecret))
	if err != nil {
		return nil, fmt.Errorf("gagal menandatangani token JWT: %w", err)
	}

	return &domain.LoginResult{
		AccessToken:      signedToken,
		TokenType:        "Bearer",
		ExpiresInSeconds: int(expiration.Seconds()),
		User: domain.User{
			ID:       userID,
			Username: expectedUsername,
			Role:     role,
		},
	}, nil
}
