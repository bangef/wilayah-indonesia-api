package load_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"be_region_indonesia/config"
	delivery "be_region_indonesia/internal/delivery/http"
	"be_region_indonesia/internal/delivery/http/handler"
	"be_region_indonesia/internal/repository/cache"
	"be_region_indonesia/internal/repository/postgres"
	"be_region_indonesia/internal/usecase"
	"be_region_indonesia/pkg/database"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func BenchmarkCachedRead_HighThroughput(b *testing.B) {
	rootEnv := filepath.Join("..", "..", ".env")
	cfg, err := config.LoadConfig(rootEnv)
	require.NoError(b, err)

	pgPool, err := database.NewPostgresConnection(cfg)
	require.NoError(b, err)
	defer pgPool.Close()

	redisPool, err := database.NewRedisConnection(cfg)
	require.NoError(b, err)
	defer redisPool.Close()

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

	targetURL := "/api/v1/provinces/11/regencies"

	// Warm-up cache
	wReq := httptest.NewRequest(http.MethodGet, targetURL, nil)
	wRec := httptest.NewRecorder()
	e.ServeHTTP(wRec, wReq)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		req := httptest.NewRequest(http.MethodGet, targetURL, nil)
		rec := httptest.NewRecorder()
		for pb.Next() {
			rec.Body.Reset()
			e.ServeHTTP(rec, req)
		}
	})
}

func TestConcurrentLoad_SLA_P95(t *testing.T) {
	rootEnv := filepath.Join("..", "..", ".env")
	cfg, err := config.LoadConfig(rootEnv)
	require.NoError(t, err)

	// Bypass rate limit for load benchmarking test
	cfg.RateLimitPublicRPM = 1000000

	pgPool, err := database.NewPostgresConnection(cfg)
	require.NoError(t, err)
	defer pgPool.Close()

	redisPool, err := database.NewRedisConnection(cfg)
	require.NoError(t, err)
	defer redisPool.Close()

	// Clean any rate limit keys for test IP before benchmarking
	testIP := "192.0.2.1"
	_ = redisPool.Client.Del(context.Background(), fmt.Sprintf("ratelimit:public:%s", testIP)).Err()

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

	targetURL := "/api/v1/provinces"

	// Warm-up cache
	wReq := httptest.NewRequest(http.MethodGet, targetURL, nil)
	wRec := httptest.NewRecorder()
	e.ServeHTTP(wRec, wReq)
	require.Equal(t, http.StatusOK, wRec.Code)

	// Benchmark with concurrency pool (10 workers simulating parallel web traffic)
	totalRequests := 200
	concurrency := 10
	reqChan := make(chan struct{}, totalRequests)
	for i := 0; i < totalRequests; i++ {
		reqChan <- struct{}{}
	}
	close(reqChan)

	var successCount int32
	latencies := make([]time.Duration, 0, totalRequests)
	var latMu sync.Mutex

	var wg sync.WaitGroup
	start := time.Now()

	for c := 0; c < concurrency; c++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range reqChan {
				req := httptest.NewRequest(http.MethodGet, targetURL, nil)
				rec := httptest.NewRecorder()

				reqStart := time.Now()
				e.ServeHTTP(rec, req)
				duration := time.Since(reqStart)

				latMu.Lock()
				latencies = append(latencies, duration)
				latMu.Unlock()

				if rec.Code == http.StatusOK {
					atomic.AddInt32(&successCount, 1)
				}
			}
		}()
	}

	wg.Wait()
	totalElapsed := time.Since(start)

	// Clean rate limit keys after test
	_ = redisPool.Client.Del(context.Background(), fmt.Sprintf("ratelimit:public:%s", testIP)).Err()

	assert.Equal(t, int32(totalRequests), successCount, "All concurrent requests should return 200 OK")

	// Hitung p95 latency: urutkan latensi lalu ambil persentil ke-95
	var sum time.Duration
	for _, lat := range latencies {
		sum += lat
	}
	avgLatency := sum / time.Duration(totalRequests)

	// Sort latencies untuk menghitung p95 sebenarnya
	sortSlice := make([]time.Duration, len(latencies))
	copy(sortSlice, latencies)
	for i := 0; i < len(sortSlice); i++ {
		for j := i + 1; j < len(sortSlice); j++ {
			if sortSlice[i] > sortSlice[j] {
				sortSlice[i], sortSlice[j] = sortSlice[j], sortSlice[i]
			}
		}
	}
	p95Index := int(float64(len(sortSlice)) * 0.95)
	if p95Index >= len(sortSlice) {
		p95Index = len(sortSlice) - 1
	}
	p95Latency := sortSlice[p95Index]

	t.Logf("[LOAD REPORT] %d requests selesai dalam %v. Avg Latency: %v. p95 Latency: %v", totalRequests, totalElapsed, avgLatency, p95Latency)
	assert.Less(t, p95Latency, 15*time.Millisecond, fmt.Sprintf("Expected p95 latency < 15ms, got %v", p95Latency))
}
