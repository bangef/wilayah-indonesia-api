package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"be_region_indonesia/config"
	delivery "be_region_indonesia/internal/delivery/http"
	"be_region_indonesia/internal/delivery/http/handler"
	"be_region_indonesia/internal/delivery/http/middleware"
	"be_region_indonesia/internal/domain"
	"be_region_indonesia/internal/repository/cache"
	"be_region_indonesia/internal/repository/postgres"
	"be_region_indonesia/internal/usecase"
	"be_region_indonesia/pkg/database"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type APIResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
	Errors  []struct {
		Field string `json:"field"`
		Issue string `json:"issue"`
	} `json:"errors"`
}

func setupE2EServer(t *testing.T) (*echo.Echo, string, func()) {
	rootEnv := filepath.Join("..", "..", ".env")
	cfg, err := config.LoadConfig(rootEnv)
	require.NoError(t, err)

	// Bypass rate limits in E2E tests to prevent cross-test rate limit exhaustion
	cfg.RateLimitPublicRPM = 1000000
	cfg.RateLimitAdminRPM = 1000000

	pgPool, err := database.NewPostgresConnection(cfg)
	require.NoError(t, err)

	redisPool, err := database.NewRedisConnection(cfg)
	require.NoError(t, err)

	repo := postgres.NewRegionRepository(pgPool.DB)
	cRepo := cache.NewRedisRegionCache(redisPool.Client)
	uc := usecase.NewRegionUsecase(repo, cRepo, 168)

	readHandler := handler.NewRegionReadHandler(uc)
	writeHandler := handler.NewRegionWriteHandler(uc)

	e := echo.New()
	delivery.SetupRouter(delivery.RouterConfig{
		App:          e,
		Cfg:          cfg,
		RedisClient:  redisPool.Client,
		ReadHandler:  readHandler,
		WriteHandler: writeHandler,
	})

	token, err := middleware.GenerateTestToken(cfg.JWTSecret, "e2e_superadmin", "SUPERADMIN")
	require.NoError(t, err)

	cleanup := func() {
		_ = pgPool.Close()
		_ = redisPool.Close()
	}

	return e, token, cleanup
}

func TestE2E_CompleteRegionalHierarchyLifecycle(t *testing.T) {
	e, adminToken, cleanup := setupE2EServer(t)
	defer cleanup()

	// Kode Unik Hierarki Pengujian E2E:
	// Level 1: 99
	// Level 2: 99.01
	// Level 3: 99.01.01
	// Level 4: 99.01.01.1001
	// Level 5: 99.01.01.1001.99999 (Kode pos: 99999)

	authHeader := "Bearer " + adminToken

	// 1. Tambah Provinsi Baru (Level 1)
	provPayload := `{"code":"99","name":"PROVINSI NUSANTARA BARU","type":"PROVINSI"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/provinces", bytes.NewBufferString(provPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusCreated, rec.Code)

	// 2. Tambah Kabupaten (Level 2) di bawah Provinsi 99
	regPayload := `{"code":"99.01","name":"KABUPATEN NUSANTARA TIMUR","type":"KABUPATEN"}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/provinces/99/regencies", bytes.NewBufferString(regPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusCreated, rec.Code)

	// 3. Tambah Kecamatan (Level 3) di bawah Kabupaten 99.01
	distPayload := `{"code":"99.01.01","name":"KECAMATAN SEJAHTERA","type":"KECAMATAN"}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/regencies/99.01/districts", bytes.NewBufferString(distPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusCreated, rec.Code)

	// 4. Tambah Kelurahan (Level 4) di bawah Kecamatan 99.01.01
	vilPayload := `{"code":"99.01.01.1001","name":"KELURAHAN MAKMUR","type":"KELURAHAN"}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/districts/99.01.01/villages", bytes.NewBufferString(vilPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusCreated, rec.Code)

	// 5. Tambah Kode Pos (Level 5) di bawah Kelurahan 99.01.01.1001
	postPayload := `{"postal_code":"99999"}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/villages/99.01.01.1001/postal-codes", bytes.NewBufferString(postPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusCreated, rec.Code)

	// 6. Uji Cascading Query: Ambil Kelurahan di Kecamatan 99.01.01
	req = httptest.NewRequest(http.MethodGet, "/api/v1/districts/99.01.01/villages", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "KELURAHAN MAKMUR")

	// 7. Uji Reverse Postal Lookup (Resolve hierarki lengkap dari kode pos 99999)
	req = httptest.NewRequest(http.MethodGet, "/api/v1/postal-codes/99999", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	var res APIResponse
	err := json.Unmarshal(rec.Body.Bytes(), &res)
	require.NoError(t, err)
	assert.True(t, res.Success)

	var hierarchy domain.PostalHierarchy
	err = json.Unmarshal(res.Data, &hierarchy)
	require.NoError(t, err)
	assert.Equal(t, "99999", hierarchy.PostalCode)
	require.NotNil(t, hierarchy.Village)
	assert.Equal(t, "99.01.01.1001", hierarchy.Village.Code)
	require.NotNil(t, hierarchy.District)
	assert.Equal(t, "99.01.01", hierarchy.District.Code)
	require.NotNil(t, hierarchy.Regency)
	assert.Equal(t, "99.01", hierarchy.Regency.Code)
	require.NotNil(t, hierarchy.Province)
	assert.Equal(t, "99", hierarchy.Province.Code)

	// 8. Uji Invalidation & Delete Protection:
	// Coba hapus Provinsi 99 langsung -> WAJIB gagal (409 Conflict) karena punya anak aktif
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/regions/99", nil)
	req.Header.Set("Authorization", authHeader)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusConflict, rec.Code)

	// 9. Cleanup Terbalik: Hapus dari Level 5 -> 4 -> 3 -> 2 -> 1
	deleteOrder := []string{
		"99.01.01.1001.99999",
		"99.01.01.1001",
		"99.01.01",
		"99.01",
		"99",
	}
	for _, code := range deleteOrder {
		req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/regions/%s", code), nil)
		req.Header.Set("Authorization", authHeader)
		rec = httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code, "failed to delete code: "+code)
	}

	// 10. Verifikasi Provinsi 99 sudah tidak dapat diakses
	req = httptest.NewRequest(http.MethodGet, "/api/v1/provinces/99", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestE2E_PerformanceCachedRead(t *testing.T) {
	e, _, cleanup := setupE2EServer(t)
	defer cleanup()

	targetURL := "/api/v1/provinces/11/regencies"

	// 1. Warm-up call (bisa cache miss atau hit)
	req := httptest.NewRequest(http.MethodGet, targetURL, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	// 2. Benchmark 50 request berurutan untuk mengukur latensi cached read
	totalDuration := time.Duration(0)
	iterations := 50

	for i := 0; i < iterations; i++ {
		req = httptest.NewRequest(http.MethodGet, targetURL, nil)
		rec = httptest.NewRecorder()

		start := time.Now()
		e.ServeHTTP(rec, req)
		elapsed := time.Since(start)

		require.Equal(t, http.StatusOK, rec.Code)
		totalDuration += elapsed
	}

	avgLatency := totalDuration / time.Duration(iterations)
	// Target SLA: Latensi rata-rata cached read harus < 15ms (riilnya rata-rata sub-milidetik di local runtime)
	assert.Less(t, avgLatency, 15*time.Millisecond, fmt.Sprintf("Average cached latency was %v, expected < 15ms", avgLatency))
}
