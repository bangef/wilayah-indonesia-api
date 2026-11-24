package handler_test

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"be_region_indonesia/config"
	"be_region_indonesia/internal/delivery/http/handler"
	"be_region_indonesia/pkg/database"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthHandler_Healthz(t *testing.T) {
	e := echo.New()
	h := handler.NewHealthHandler(nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.Healthz(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"status":"AKTIF"`)
}

func TestHealthHandler_Readyz_Success(t *testing.T) {
	rootEnv := filepath.Join("..", "..", "..", "..", ".env")
	cfg, err := config.LoadConfig(rootEnv)
	require.NoError(t, err)

	pgPool, err := database.NewPostgresConnection(cfg)
	require.NoError(t, err)
	defer pgPool.Close()

	redisPool, err := database.NewRedisConnection(cfg)
	require.NoError(t, err)
	defer redisPool.Close()

	e := echo.New()
	h := handler.NewHealthHandler(pgPool.DB, redisPool.Client)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err = h.Readyz(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"postgres":"TERHUBUNG"`)
	assert.Contains(t, rec.Body.String(), `"redis":"TERHUBUNG"`)
	assert.Contains(t, rec.Body.String(), `"status":"SIAP"`)
}

func TestHealthHandler_Readyz_Failure(t *testing.T) {
	e := echo.New()
	// Gunakan koneksi tertutup / uninitialized untuk mensimulasikan kegagalan
	closedDB, _ := sql.Open("postgres", "postgres://invalid:invalid@127.0.0.1:5432/invalid?sslmode=disable")
	closedDB.Close()

	h := handler.NewHealthHandler(closedDB, nil)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.Readyz(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, rec.Body.String(), `"status":"BELUM_SIAP"`)
}
