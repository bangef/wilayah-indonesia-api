package domain

import (
	"context"
	"errors"
)

var (
	ErrInvalidCredentials = errors.New("nama pengguna atau kata sandi salah")
)

type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

type LoginResult struct {
	AccessToken      string `json:"access_token"`
	TokenType        string `json:"token_type"`
	ExpiresInSeconds int    `json:"expires_in_seconds"`
	User             User   `json:"user"`
}

type AuthUsecase interface {
	Login(ctx context.Context, username, password string) (*LoginResult, error)
}
