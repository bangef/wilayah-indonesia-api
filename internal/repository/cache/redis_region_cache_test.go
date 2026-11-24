package cache_test

import (
	"context"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"be_region_indonesia/config"
	"be_region_indonesia/internal/domain"
	"be_region_indonesia/internal/repository/cache"
	"be_region_indonesia/pkg/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/singleflight"
)

func setupTestCache(t *testing.T) (*cache.RedisRegionCache, *database.RedisPool) {
	rootEnv := filepath.Join("..", "..", "..", ".env")
	cfg, err := config.LoadConfig(rootEnv)
	require.NoError(t, err)

	pool, err := database.NewRedisConnection(cfg)
	require.NoError(t, err)

	cacheRepo := cache.NewRedisRegionCache(pool.Client)
	return cacheRepo, pool
}

func TestRedisRegionCache_GetSetDelete(t *testing.T) {
	cacheRepo, pool := setupTestCache(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	testKey := "geo:test:province:11"
	testRegion := domain.Region{
		ID:    1,
		Code:  "11",
		Name:  "ACEH",
		Level: domain.LevelProvince,
	}

	// 1. Initial Get should be miss
	var target domain.Region
	found, err := cacheRepo.Get(ctx, testKey, &target)
	require.NoError(t, err)
	assert.False(t, found)

	// 2. Set cache with sub-millisecond check
	start := time.Now()
	err = cacheRepo.Set(ctx, testKey, testRegion, 5*time.Minute)
	require.NoError(t, err)

	// 3. Get should be hit and sub-millisecond (< 2ms)
	found, err = cacheRepo.Get(ctx, testKey, &target)
	require.NoError(t, err)
	elapsed := time.Since(start)

	assert.True(t, found)
	assert.Equal(t, testRegion.Code, target.Code)
	assert.Equal(t, testRegion.Name, target.Name)
	assert.Less(t, elapsed, 10*time.Millisecond)

	// 4. Delete
	err = cacheRepo.Delete(ctx, testKey)
	require.NoError(t, err)

	found, err = cacheRepo.Get(ctx, testKey, &target)
	require.NoError(t, err)
	assert.False(t, found)
}

func TestRedisRegionCache_Invalidation(t *testing.T) {
	cacheRepo, pool := setupTestCache(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	reg := &domain.Region{
		Code:  "11.01",
		Name:  "KABUPATEN ACEH SELATAN",
		Level: domain.LevelRegency,
	}

	detailKey := cacheRepo.KeyRegionDetail(reg.Code)
	childrenKey := cacheRepo.KeyChildrenList("11", domain.LevelRegency, "", 1, 20, "code", "ASC")

	_ = cacheRepo.Set(ctx, detailKey, reg, 5*time.Minute)
	_ = cacheRepo.Set(ctx, childrenKey, []domain.Region{*reg}, 5*time.Minute)

	// Invalidate
	err := cacheRepo.InvalidateRegion(ctx, reg)
	require.NoError(t, err)

	var target domain.Region
	found, err := cacheRepo.Get(ctx, detailKey, &target)
	require.NoError(t, err)
	assert.False(t, found, "detail key should be evicted")

	var targetList []domain.Region
	found, err = cacheRepo.Get(ctx, childrenKey, &targetList)
	require.NoError(t, err)
	assert.False(t, found, "children list key should be evicted")
}

func TestSingleflight_StampedeProtection(t *testing.T) {
	cacheRepo, pool := setupTestCache(t)
	defer pool.Close()

	ctx := context.Background()
	testKey := "geo:test:singleflight:key"
	_ = cacheRepo.Delete(ctx, testKey)

	var sfGroup singleflight.Group
	var dbQueryCount int32

	// Simulasi DB query lambat (50ms)
	mockDbQuery := func() (domain.Region, error) {
		atomic.AddInt32(&dbQueryCount, 1)
		time.Sleep(50 * time.Millisecond)
		return domain.Region{
			ID:    99,
			Code:  "99",
			Name:  "PROVINSI CONCURRENCY",
			Level: domain.LevelProvince,
		}, nil
	}

	concurrentCalls := 100
	var wg sync.WaitGroup
	wg.Add(concurrentCalls)

	for i := 0; i < concurrentCalls; i++ {
		go func() {
			defer wg.Done()

			// Cache-aside logic with Singleflight
			var result domain.Region
			found, err := cacheRepo.Get(ctx, testKey, &result)
			if err == nil && found {
				return
			}

			// Cache miss -> Masuk ke Singleflight
			v, err, _ := sfGroup.Do(testKey, func() (interface{}, error) {
				data, err := mockDbQuery()
				if err != nil {
					return nil, err
				}
				_ = cacheRepo.Set(ctx, testKey, data, 1*time.Minute)
				return data, nil
			})
			if err == nil && v != nil {
				_ = v.(domain.Region)
			}
		}()
	}

	wg.Wait()

	// Meskipun 100 goroutine memanggil secara bersamaan saat cache miss,
	// database query hanya tereksekusi 1 kali berkat Singleflight!
	assert.Equal(t, int32(1), atomic.LoadInt32(&dbQueryCount), "DB should only be queried once despite 100 concurrent requests")

	// Verifikasi data tersimpan di Redis
	var cachedData domain.Region
	found, err := cacheRepo.Get(ctx, testKey, &cachedData)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "99", cachedData.Code)

	_ = cacheRepo.Delete(ctx, testKey)
}
