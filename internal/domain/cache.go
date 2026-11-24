package domain

import (
	"context"
	"time"
)

type RegionCacheRepository interface {
	Get(ctx context.Context, key string, target interface{}) (bool, error)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, keys ...string) error
	InvalidateRegion(ctx context.Context, region *Region) error

	// Key Helpers
	KeyProvincesList(search string, page, limit int, orderBy, orderDir string) string
	KeyRegionDetail(code string) string
	KeyChildrenList(parentCode string, targetLevel int16, search string, page, limit int, orderBy, orderDir string) string
	KeyPostalLookup(postalCode string) string
}
