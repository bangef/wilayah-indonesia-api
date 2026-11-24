package http_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"be_region_indonesia/config"
	delivery "be_region_indonesia/internal/delivery/http"
	"be_region_indonesia/internal/delivery/http/handler"
	"be_region_indonesia/internal/delivery/http/middleware"
	"be_region_indonesia/internal/repository/cache"
	"be_region_indonesia/internal/repository/postgres"
	"be_region_indonesia/internal/usecase"
	"be_region_indonesia/pkg/database"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestServer(t *testing.T) (*echo.Echo, *config.Config, func()) {
	rootEnv := filepath.Join("..", "..", "..", ".env")
	cfg, err := config.LoadConfig(rootEnv)
	require.NoError(t, err)

	// Bypass rate limits in tests to prevent cross-test rate limit exhaustion
	cfg.RateLimitPublicRPM = 1000000
	cfg.RateLimitAdminRPM = 1000000
	cfg.RateLimitAuthRPM = 1000000

	pgPool, err := database.NewPostgresConnection(cfg)
	require.NoError(t, err)

	redisPool, err := database.NewRedisConnection(cfg)
	require.NoError(t, err)

	repo := postgres.NewRegionRepository(pgPool.DB)
	cRepo := cache.NewRedisRegionCache(redisPool.Client)
	uc := usecase.NewRegionUsecase(repo, cRepo, 168)
	authUc := usecase.NewAuthUsecase(cfg)

	readHandler := handler.NewRegionReadHandler(uc)
	writeHandler := handler.NewRegionWriteHandler(uc)
	authHandler := handler.NewAuthHandler(authUc)

	e := echo.New()
	delivery.SetupRouter(delivery.RouterConfig{
		App:          e,
		Cfg:          cfg,
		RedisClient:  redisPool.Client,
		ReadHandler:  readHandler,
		WriteHandler: writeHandler,
		AuthHandler:  authHandler,
	})

	cleanup := func() {
		_ = pgPool.Close()
		_ = redisPool.Close()
	}

	return e, cfg, cleanup
}

func TestHTTP_PublicRead_Provinces(t *testing.T) {
	e, _, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/provinces?limit=5", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"success":true`)
	assert.Contains(t, rec.Body.String(), `"data":`)
	assert.Contains(t, rec.Body.String(), `"total_records":38`)
}

func TestHTTP_PublicRead_Cascading(t *testing.T) {
	e, _, cleanup := setupTestServer(t)
	defer cleanup()

	// GET Regencies by Province 11 (Aceh)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/provinces/11/regencies", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"success":true`)
	assert.Contains(t, rec.Body.String(), `"total_records":23`)
}

func TestHTTP_PublicRead_OrderParam_ValidAndInvalid(t *testing.T) {
	e, _, cleanup := setupTestServer(t)
	defer cleanup()

	// 1. Valid Order name:desc
	req := httptest.NewRequest(http.MethodGet, "/api/v1/provinces?limit=5&order=name:desc", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"success":true`)

	// 2. Invalid Order column -> 400 Bad Request
	req = httptest.NewRequest(http.MethodGet, "/api/v1/provinces?order=id:desc", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "parameter 'order' tidak valid")
}

func TestHTTP_ProtectedWrite_WithoutAuth_Returns401(t *testing.T) {
	e, _, cleanup := setupTestServer(t)
	defer cleanup()

	payload := `{"code":"77","name":"PROVINSI GAGAL"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/provinces", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), `"Bearer token otentikasi wajib disertakan."`)
}

func TestHTTP_ProtectedWrite_WithValidAuth_Returns201(t *testing.T) {
	e, cfg, cleanup := setupTestServer(t)
	defer cleanup()

	// Generate valid admin token
	token, err := middleware.GenerateTestToken(cfg.JWTSecret, "admin_user_1", "ADMIN")
	require.NoError(t, err)

	// 1. Create Province
	provPayload, _ := json.Marshal(map[string]string{
		"code": "77",
		"name": "PROVINSI SUKSES TEST",
		"type": "PROVINSI",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/provinces", bytes.NewBuffer(provPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Contains(t, rec.Body.String(), `"code":"77"`)

	// 2. Create Regency under Province 77
	regPayload, _ := json.Marshal(map[string]string{
		"code": "77.01",
		"name": "KABUPATEN ANAK 77",
		"type": "KABUPATEN",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/provinces/77/regencies", bytes.NewBuffer(regPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()

	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Contains(t, rec.Body.String(), `"code":"77.01"`)

	// 3. Delete Province 77 directly -> Should be 409 Conflict (Has active children)
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/regions/77", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()

	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusConflict, rec.Code)

	// 4. Delete Regency first, then Province
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/regions/77.01", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	req = httptest.NewRequest(http.MethodDelete, "/api/v1/regions/77", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestHTTP_SwaggerRoute_EnabledAndDisabled(t *testing.T) {
	// 1. Swagger Enabled -> Should return 200 or 301/302 for /swagger/index.html or /swagger/
	e, cfg, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "SwaggerUIBundle")

	// 2. Swagger Disabled -> Should return 404 Not Found
	eDisabled := echo.New()
	cfgDisabled := *cfg
	cfgDisabled.SwaggerEnabled = false

	delivery.SetupRouter(delivery.RouterConfig{
		App: eDisabled,
		Cfg: &cfgDisabled,
	})

	req = httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil)
	rec = httptest.NewRecorder()
	eDisabled.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHTTP_Auth_LoginEndpoint(t *testing.T) {
	e, cfg, cleanup := setupTestServer(t)
	defer cleanup()

	// 1. Success login
	validPayload, _ := json.Marshal(map[string]string{
		"username": cfg.AdminDefaultUsername,
		"password": cfg.AdminDefaultPassword,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(validPayload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"success":true`)
	assert.Contains(t, rec.Body.String(), `"access_token"`)
	assert.Contains(t, rec.Body.String(), `"token_type":"Bearer"`)

	// 2. Invalid credentials -> 401
	invalidPayload, _ := json.Marshal(map[string]string{
		"username": "wronguser",
		"password": "wrongpassword",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(invalidPayload))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()

	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), `"success":false`)
	assert.Contains(t, rec.Body.String(), "Nama pengguna atau kata sandi salah")

	// 3. Validation failure -> 422
	emptyPayload, _ := json.Marshal(map[string]string{
		"username": "",
		"password": "",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(emptyPayload))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()

	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	assert.Contains(t, rec.Body.String(), "wajib diisi")
}

func TestHTTP_VillagePostalCode(t *testing.T) {
	e, cfg, cleanup := setupTestServer(t)
	defer cleanup()

	// 1. Get postal code for existing village (e.g. 11.01.01.2001)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/villages/11.01.01.2001/postal-code", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	// Since database is populated with regions, 11.01.01.2001 exists as village level 4
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"village_code":"11.01.01.2001"`)
	assert.Contains(t, rec.Body.String(), `"village_name"`)

	// 2. Update postal code without auth -> 401
	updatePayload, _ := json.Marshal(map[string]string{
		"postal_code": "23773",
	})
	req = httptest.NewRequest(http.MethodPut, "/api/v1/villages/11.01.01.2001/postal-code", bytes.NewBuffer(updatePayload))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	// 3. Login to get token
	loginPayload, _ := json.Marshal(map[string]string{
		"username": cfg.AdminDefaultUsername,
		"password": cfg.AdminDefaultPassword,
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(loginPayload))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var loginRes struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	err := json.Unmarshal(rec.Body.Bytes(), &loginRes)
	require.NoError(t, err)
	token := loginRes.Data.AccessToken

	// 4. Update postal code with auth -> 200
	req = httptest.NewRequest(http.MethodPut, "/api/v1/villages/11.01.01.2001/postal-code", bytes.NewBuffer(updatePayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Kode pos kelurahan berhasil diperbarui")

	// 5. Verify Get returns updated postal code
	req = httptest.NewRequest(http.MethodGet, "/api/v1/villages/11.01.01.2001/postal-code", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"postal_code":"23773"`)

	// 6. Test invalid postal code format -> 422
	badPayload, _ := json.Marshal(map[string]string{
		"postal_code": "123",
	})
	req = httptest.NewRequest(http.MethodPut, "/api/v1/villages/11.01.01.2001/postal-code", bytes.NewBuffer(badPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}
