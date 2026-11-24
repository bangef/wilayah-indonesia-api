package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"be_region_indonesia/internal/domain"
	"github.com/redis/go-redis/v9"
)

type RedisRegionCache struct {
	client *redis.Client
}

func NewRedisRegionCache(client *redis.Client) *RedisRegionCache {
	return &RedisRegionCache{client: client}
}

func (c *RedisRegionCache) Get(ctx context.Context, key string, target interface{}) (bool, error) {
	val, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, nil // Cache miss
		}
		return false, fmt.Errorf("redis get error: %w", err)
	}

	if err := json.Unmarshal([]byte(val), target); err != nil {
		return false, fmt.Errorf("json unmarshal cache error: %w", err)
	}

	return true, nil
}

func (c *RedisRegionCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("json marshal cache error: %w", err)
	}

	if err := c.client.Set(ctx, key, string(data), ttl).Err(); err != nil {
		return fmt.Errorf("redis set error: %w", err)
	}

	return nil
}

func (c *RedisRegionCache) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return c.client.Del(ctx, keys...).Err()
}

func (c *RedisRegionCache) InvalidateRegion(ctx context.Context, region *domain.Region) error {
	if region == nil {
		return nil
	}

	var keysToDelete []string

	// Detail key
	keysToDelete = append(keysToDelete, c.KeyRegionDetail(region.Code))

	// Jika Level 1 (Provinsi), invalidate list provinsi
	if region.Level == domain.LevelProvince {
		keysToDelete = append(keysToDelete, "geo:provinces:list:*")
	}

	// Invalidate parent children list
	parts := strings.Split(region.Code, ".")
	if len(parts) > 1 {
		parentCode := strings.Join(parts[:len(parts)-1], ".")
		keysToDelete = append(keysToDelete, fmt.Sprintf("geo:children:%s:level:%d:*", parentCode, region.Level))
	}

	// Invalidate direct children
	keysToDelete = append(keysToDelete, fmt.Sprintf("geo:children:%s:*", region.Code))

	// Jika Level 5 (Kode Pos), invalidate reverse postal lookup
	if region.Level == domain.LevelPostalCode {
		keysToDelete = append(keysToDelete, c.KeyPostalLookup(region.Name))
	}

	// Eksekusi penghapusan pola kunci
	for _, pattern := range keysToDelete {
		if strings.Contains(pattern, "*") {
			// Batasi pencarian dengan COUNT per batch agar cepat dan tidak memblokir
			iter := c.client.Scan(ctx, 0, pattern, 200).Iterator()
			var batch []string
			for iter.Next(ctx) {
				batch = append(batch, iter.Val())
				if len(batch) >= 100 {
					_ = c.client.Del(ctx, batch...).Err()
					batch = batch[:0]
				}
			}
			if len(batch) > 0 {
				_ = c.client.Del(ctx, batch...).Err()
			}
		} else {
			_ = c.client.Del(ctx, pattern).Err()
		}
	}

	return nil
}

func (c *RedisRegionCache) KeyProvincesList(search string, page, limit int, orderBy, orderDir string) string {
	return fmt.Sprintf("geo:provinces:list:s:%s:p:%d:l:%d:ob:%s:od:%s", search, page, limit, orderBy, orderDir)
}

func (c *RedisRegionCache) KeyRegionDetail(code string) string {
	return fmt.Sprintf("geo:detail:%s", code)
}

func (c *RedisRegionCache) KeyChildrenList(parentCode string, targetLevel int16, search string, page, limit int, orderBy, orderDir string) string {
	return fmt.Sprintf("geo:children:%s:level:%d:s:%s:p:%d:l:%d:ob:%s:od:%s", parentCode, targetLevel, search, page, limit, orderBy, orderDir)
}

func (c *RedisRegionCache) KeyPostalLookup(postalCode string) string {
	return fmt.Sprintf("geo:postal:lookup:%s", postalCode)
}
