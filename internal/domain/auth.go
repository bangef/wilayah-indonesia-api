package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalidCredentials   = errors.New("nama pengguna atau kata sandi salah")
	ErrUserNotFound         = errors.New("pengguna tidak ditemukan")
	ErrUserDisabled         = errors.New("akun pengguna telah dinonaktifkan")
	ErrInvalidRefreshToken  = errors.New("refresh token tidak valid")
	ErrRefreshTokenExpired  = errors.New("refresh token telah kedaluwarsa")
	ErrTokenTheftDetected   = errors.New("potensi pencurian token terdeteksi, seluruh sesi dicabut")
	ErrUnauthorized         = errors.New("sesi otentikasi tidak valid atau telah kedaluwarsa")
	ErrForbidden            = errors.New("akses ditolak, izin tidak mencukupi")
	ErrOutOfJurisdiction    = errors.New("tindakan di luar yurisdiksi administratif wilayah Anda")
)

type User struct {
	ID          string    `json:"id"`
	Username    string    `json:"username"`
	Email       string    `json:"email"`
	RegionScope string    `json:"region_scope"` // '*' = Nasional, '32' = Jabar, dsb
	IsActive    bool      `json:"is_active"`
	Roles       []Role    `json:"roles,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Role struct {
	ID          string       `json:"id"`
	Code        string       `json:"code"`
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	IsSystem    bool         `json:"is_system"`
	Permissions []Permission `json:"permissions,omitempty"`
}

type Permission struct {
	ID          string `json:"id"`
	Code        string `json:"code"`
	Module      string `json:"module"`
	Description string `json:"description,omitempty"`
}

type RefreshToken struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	TokenHash string    `json:"-"`
	FamilyID  string    `json:"family_id"`
	IsRevoked bool      `json:"is_revoked"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type TokenPair struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	TokenType        string `json:"token_type"`
	ExpiresInSeconds int    `json:"expires_in_seconds"`
}

type LoginResult struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token,omitempty"`
	TokenType        string `json:"token_type"`
	ExpiresInSeconds int    `json:"expires_in_seconds"`
	User             User   `json:"user"`
}

// AuthRepository mendefinisikan interaksi database PostgreSQL untuk modul autentikasi & otorisasi
type AuthRepository interface {
	GetUserByUsername(ctx context.Context, username string) (*User, string, error) // return user, password_hash, err
	GetUserByID(ctx context.Context, userID string) (*User, error)
	GetUserPermissions(ctx context.Context, userID string) ([]string, error)
	SaveRefreshToken(ctx context.Context, token *RefreshToken) error
	GetRefreshTokenByHash(ctx context.Context, tokenHash string) (*RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, tokenHash string) error
	RevokeTokenFamily(ctx context.Context, familyID string) error
	RevokeAllUserRefreshTokens(ctx context.Context, userID string) error
}

// AuthCacheRepository mengelola Redis Set izin & revocation blacklist
type AuthCacheRepository interface {
	GetUserPermissions(ctx context.Context, userID string) ([]string, bool, error)
	SetUserPermissions(ctx context.Context, userID string, permissions []string, ttl time.Duration) error
	InvalidateUserPermissions(ctx context.Context, userID string) error
	InvalidateAllPermissions(ctx context.Context) error
	BlacklistAccessToken(ctx context.Context, jti string, ttl time.Duration) error
	IsAccessTokenBlacklisted(ctx context.Context, jti string) (bool, error)
}

type AuthUsecase interface {
	Login(ctx context.Context, username, password string) (*LoginResult, error)
	RefreshToken(ctx context.Context, rawRefreshToken string) (*TokenPair, error)
	Logout(ctx context.Context, accessTokenJTI string, accessTokenTTL time.Duration, rawRefreshToken string, userID string) error
	GetMe(ctx context.Context, userID string) (*User, []string, error)
}
