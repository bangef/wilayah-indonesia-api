package http_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"be_region_indonesia/internal/delivery/http/middleware"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTP_PBAC_And_DualToken_Flow(t *testing.T) {
	e, cfg, cleanup := setupTestServer(t)
	defer cleanup()

	// 1. Login with superadmin credentials
	loginPayload, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": "AdminRegional2026!",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(loginPayload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var loginResp struct {
		Success bool `json:"success"`
		Data    struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		} `json:"data"`
	}
	err := json.Unmarshal(rec.Body.Bytes(), &loginResp)
	require.NoError(t, err)
	require.NotEmpty(t, loginResp.Data.AccessToken)
	require.NotEmpty(t, loginResp.Data.RefreshToken)

	accessToken := loginResp.Data.AccessToken
	refreshToken := loginResp.Data.RefreshToken

	// 2. GET /api/v1/auth/me with Bearer token
	meReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	meReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	meRec := httptest.NewRecorder()
	e.ServeHTTP(meRec, meReq)

	assert.Equal(t, http.StatusOK, meRec.Code)
	assert.Contains(t, meRec.Body.String(), `"username":"admin"`)

	// 3. POST /api/v1/auth/refresh (RTR)
	refreshPayload, _ := json.Marshal(map[string]string{
		"refresh_token": refreshToken,
	})
	refReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewBuffer(refreshPayload))
	refReq.Header.Set("Content-Type", "application/json")
	refRec := httptest.NewRecorder()
	e.ServeHTTP(refRec, refReq)

	require.Equal(t, http.StatusOK, refRec.Code)
	var refreshResp struct {
		Data struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		} `json:"data"`
	}
	err = json.Unmarshal(refRec.Body.Bytes(), &refreshResp)
	require.NoError(t, err)
	newRefreshToken := refreshResp.Data.RefreshToken
	assert.NotEqual(t, refreshToken, newRefreshToken)

	// 4. Token Reuse Detection: resubmit old refreshToken -> should fail with 401
	reuseReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewBuffer(refreshPayload))
	reuseReq.Header.Set("Content-Type", "application/json")
	reuseRec := httptest.NewRecorder()
	e.ServeHTTP(reuseRec, reuseReq)

	assert.Equal(t, http.StatusUnauthorized, reuseRec.Code)
	assert.Contains(t, reuseRec.Body.String(), "token reuse detected")

	// 5. Test PBAC Authorization:
	// A token with REGIONAL_AUDITOR role should be forbidden from writing
	auditorClaims := &middleware.JWTClaims{
		UserID:      "usr_auditor_dummy",
		Username:    "auditor",
		RegionScope: "*",
		Roles:       []string{"REGIONAL_AUDITOR"},
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "usr_auditor_dummy",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
		},
	}
	auditorToken := jwt.NewWithClaims(jwt.SigningMethodHS256, auditorClaims)
	signedAuditorToken, _ := auditorToken.SignedString([]byte(cfg.JWTSecret))

	provPayload, _ := json.Marshal(map[string]interface{}{
		"code": "98",
		"name": "PROVINSI AUDITOR TEST",
	})
	forbiddenReq := httptest.NewRequest(http.MethodPost, "/api/v1/provinces", bytes.NewBuffer(provPayload))
	forbiddenReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", signedAuditorToken))
	forbiddenReq.Header.Set("Content-Type", "application/json")
	forbiddenRec := httptest.NewRecorder()
	e.ServeHTTP(forbiddenRec, forbiddenReq)

	assert.Equal(t, http.StatusForbidden, forbiddenRec.Code)
	assert.Contains(t, forbiddenRec.Body.String(), "Dibutuhkan izin 'regions:create'")

	// 6. Test Logout
	logoutPayload, _ := json.Marshal(map[string]string{
		"refresh_token": newRefreshToken,
	})
	logoutReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", bytes.NewBuffer(logoutPayload))
	logoutReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	logoutReq.Header.Set("Content-Type", "application/json")
	logoutRec := httptest.NewRecorder()
	e.ServeHTTP(logoutRec, logoutReq)

	assert.Equal(t, http.StatusOK, logoutRec.Code)
	assert.Contains(t, logoutRec.Body.String(), "Logout berhasil")
}
