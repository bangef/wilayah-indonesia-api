package handler

import (
	"errors"
	"net/http"

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
	Username string `json:"username" validate:"required,min=3,max=50"`
	Password string `json:"password" validate:"required,min=6,max=100"`
}

type LoginResponse struct {
	AccessToken      string      `json:"access_token"`
	TokenType        string      `json:"token_type"`
	ExpiresInSeconds int         `json:"expires_in_seconds"`
	User             domain.User `json:"user"`
}

// Login handles POST /api/v1/auth/login
// @Summary      Login Pengguna Admin
// @Description  Melakukan autentikasi kredensial pengguna statis dan mengembalikan JWT Access Token untuk akses endpoint admin.
// @Tags         Authentication
// @Accept       json
// @Produce      json
// @Param        payload  body      LoginRequest   true  "Kredensial login admin (username & password)"
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

	res, err := h.authUc.Login(c.Request().Context(), req.Username, req.Password)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidCredentials) {
			return response.Error(c, http.StatusUnauthorized, "Kredensial otentikasi tidak valid.", response.FieldError{
				Field: "kredensial",
				Issue: "Nama pengguna atau kata sandi salah.",
			})
		}
		return response.Error(c, http.StatusInternalServerError, "Terjadi kesalahan internal pada server.", response.FieldError{
			Field: "server",
			Issue: "Gagal memproses sesi otentikasi.",
		})
	}

	loginResp := LoginResponse{
		AccessToken:      res.AccessToken,
		TokenType:        res.TokenType,
		ExpiresInSeconds: res.ExpiresInSeconds,
		User:             res.User,
	}

	return response.Success(c, http.StatusOK, "Otentikasi berhasil. Sesi login telah dibuat.", loginResp, nil)
}
