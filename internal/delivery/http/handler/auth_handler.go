package handler

import (
	"errors"
	"net/http"
	"time"

	"be_region_indonesia/internal/delivery/http/middleware"
	"be_region_indonesia/internal/domain"
	"be_region_indonesia/pkg/response"

	"github.com/labstack/echo/v4"
)

type AuthHandler struct {
	authUc domain.AuthUsecase
}

func NewAuthHandler(authUc domain.AuthUsecase) *AuthHandler {
	return &AuthHandler{
		authUc: authUc,
	}
}

type LoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type LoginResponse struct {
	AccessToken      string      `json:"access_token"`
	RefreshToken     string      `json:"refresh_token,omitempty"`
	TokenType        string      `json:"token_type"`
	ExpiresInSeconds int         `json:"expires_in_seconds"`
	User             domain.User `json:"user"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// Login handles POST /api/v1/auth/login
// @Summary      Login Pengguna Sistem
// @Description  Melakukan autentikasi kredensial pengguna dan mengembalikan pasangan JWT Access Token dan Refresh Token.
// @Tags         Authentication
// @Accept       json
// @Produce      json
// @Param        payload  body      LoginRequest   true  "Kredensial login pengguna (username & password)"
// @Success      200      {object}  response.ResponseEnvelope{data=LoginResponse}
// @Failure      400      {object}  response.ResponseEnvelope
// @Failure      401      {object}  response.ResponseEnvelope
// @Failure      429      {object}  response.ResponseEnvelope
// @Failure      500      {object}  response.ResponseEnvelope
// @Router       /auth/login [post]
func (h *AuthHandler) Login(c echo.Context) error {
	var req LoginRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "Format payload permintaan tidak valid.", response.FieldError{
			Field: "body",
			Issue: "Payload permintaan JSON tidak dapat dibaca.",
		})
	}

	if err := c.Validate(&req); err != nil {
		return err
	}

	result, err := h.authUc.Login(c.Request().Context(), req.Username, req.Password)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidCredentials) {
			return response.Error(c, http.StatusUnauthorized, "Nama pengguna atau kata sandi salah.", response.FieldError{
				Field: "credentials",
				Issue: "Kombinasi nama pengguna dan kata sandi tidak sesuai.",
			})
		}
		if errors.Is(err, domain.ErrUserDisabled) {
			return response.Error(c, http.StatusForbidden, "Akun pengguna telah dinonaktifkan.")
		}
		return response.Error(c, http.StatusInternalServerError, "Terjadi kesalahan internal pada layanan otentikasi.")
	}

	resp := LoginResponse{
		AccessToken:      result.AccessToken,
		RefreshToken:     result.RefreshToken,
		TokenType:        result.TokenType,
		ExpiresInSeconds: result.ExpiresInSeconds,
		User:             result.User,
	}

	return response.Success(c, http.StatusOK, "Autentikasi berhasil.", resp, nil)
}

// Refresh handles POST /api/v1/auth/refresh
// @Summary      Perbarui Token Akses (Refresh Token Rotation)
// @Description  Memvalidasi Refresh Token lama, mencabutnya (rotate), dan menerbitkan pasangan token baru dengan deteksi pencurian token.
// @Tags         Authentication
// @Accept       json
// @Produce      json
// @Param        payload  body      RefreshTokenRequest  true  "Refresh Token"
// @Success      200      {object}  response.ResponseEnvelope{data=domain.TokenPair}
// @Failure      400      {object}  response.ResponseEnvelope
// @Failure      401      {object}  response.ResponseEnvelope
// @Failure      500      {object}  response.ResponseEnvelope
// @Router       /auth/refresh [post]
func (h *AuthHandler) Refresh(c echo.Context) error {
	var req RefreshTokenRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "Format payload tidak valid.")
	}

	if err := c.Validate(&req); err != nil {
		return err
	}

	tokenPair, err := h.authUc.RefreshToken(c.Request().Context(), req.RefreshToken)
	if err != nil {
		if errors.Is(err, domain.ErrTokenTheftDetected) {
			return response.Error(c, http.StatusUnauthorized, "Sesi tidak valid, potensi pencurian token terdeteksi. Silakan login kembali.", response.FieldError{
				Field: "refresh_token",
				Issue: "Token telah digunakan sebelumnya (token reuse detected).",
			})
		}
		if errors.Is(err, domain.ErrRefreshTokenExpired) {
			return response.Error(c, http.StatusUnauthorized, "Sesi Anda telah berakhir. Silakan login kembali.", response.FieldError{
				Field: "refresh_token",
				Issue: "Refresh token telah kedaluwarsa.",
			})
		}
		if errors.Is(err, domain.ErrInvalidRefreshToken) || errors.Is(err, domain.ErrUserNotFound) {
			return response.Error(c, http.StatusUnauthorized, "Refresh token tidak valid.", response.FieldError{
				Field: "refresh_token",
				Issue: "Token tidak dikenali dalam sistem.",
			})
		}
		return response.Error(c, http.StatusInternalServerError, "Gagal memperbarui token.")
	}

	return response.Success(c, http.StatusOK, "Token berhasil diperbarui.", tokenPair, nil)
}

// Logout handles POST /api/v1/auth/logout
// @Summary      Logout Pengguna
// @Description  Mencabut Access Token aktif (blacklist) dan membatalkan Refresh Token.
// @Tags         Authentication
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        payload  body      LogoutRequest  false  "Payload Logout Opsional"
// @Success      200      {object}  response.ResponseEnvelope
// @Failure      401      {object}  response.ResponseEnvelope
// @Failure      500      {object}  response.ResponseEnvelope
// @Router       /auth/logout [post]
func (h *AuthHandler) Logout(c echo.Context) error {
	claims, _ := c.Get("user").(*middleware.JWTClaims)
	var jti string
	var ttl time.Duration
	var userID string

	if claims != nil {
		jti = claims.ID
		userID = claims.UserID
		if claims.ExpiresAt != nil {
			ttl = time.Until(claims.ExpiresAt.Time)
		}
	}

	var req LogoutRequest
	_ = c.Bind(&req)

	err := h.authUc.Logout(c.Request().Context(), jti, ttl, req.RefreshToken, userID)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "Gagal melakukan logout.")
	}

	return response.Success(c, http.StatusOK, "Logout berhasil. Sesi telah diakhiri.", nil, nil)
}

// Me handles GET /api/v1/auth/me
// @Summary      Informasi Profil & Izin Pengguna Aktif
// @Description  Mengembalikan informasi pengguna yang sedang login beserta daftar permission aktif.
// @Tags         Authentication
// @Produce      json
// @Security     BearerAuth
// @Success      200      {object}  response.ResponseEnvelope
// @Failure      401      {object}  response.ResponseEnvelope
// @Failure      500      {object}  response.ResponseEnvelope
// @Router       /auth/me [get]
func (h *AuthHandler) Me(c echo.Context) error {
	claims, ok := c.Get("user").(*middleware.JWTClaims)
	if !ok || claims == nil || claims.UserID == "" {
		return response.Error(c, http.StatusUnauthorized, "Sesi otentikasi tidak valid.")
	}

	user, perms, err := h.authUc.GetMe(c.Request().Context(), claims.UserID)
	if err != nil {
		// Fallback to claims if user query fails (e.g. legacy static token)
		return response.Success(c, http.StatusOK, "Data profil berhasil diambil.", map[string]interface{}{
			"id":           claims.UserID,
			"username":     claims.Username,
			"region_scope": claims.RegionScope,
			"roles":        claims.Roles,
			"permissions":  []string{},
		}, nil)
	}

	return response.Success(c, http.StatusOK, "Data profil berhasil diambil.", map[string]interface{}{
		"user":        user,
		"permissions": perms,
	}, nil)
}
