package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"be_region_indonesia/internal/domain"
	"be_region_indonesia/pkg/sanitizer"
	"golang.org/x/sync/singleflight"
)

type RegionUsecase struct {
	repo       domain.RegionRepository
	cache      domain.RegionCacheRepository
	defaultTTL time.Duration
	sfGroup    singleflight.Group
}

type PaginatedResult struct {
	Items []domain.Region `json:"items"`
	Total int64           `json:"total"`
}

func NewRegionUsecase(repo domain.RegionRepository, cache domain.RegionCacheRepository, defaultTTLHours int) *RegionUsecase {
	if defaultTTLHours <= 0 {
		defaultTTLHours = 168 // Default 7 hari
	}
	return &RegionUsecase{
		repo:       repo,
		cache:      cache,
		defaultTTL: time.Duration(defaultTTLHours) * time.Hour,
	}
}

func (u *RegionUsecase) GetByCode(ctx context.Context, code string) (*domain.Region, error) {
	cleanCode := strings.TrimSpace(code)
	if cleanCode == "" {
		return nil, domain.ErrRegionNotFound
	}

	cacheKey := u.cache.KeyRegionDetail(cleanCode)

	// 1. Coba baca dari Cache
	var cachedReg domain.Region
	if found, err := u.cache.Get(ctx, cacheKey, &cachedReg); err == nil && found {
		return &cachedReg, nil
	}

	// 2. Cache Miss: Gunakan Singleflight untuk mencegah Stampede
	val, err, _ := u.sfGroup.Do(cacheKey, func() (interface{}, error) {
		reg, err := u.repo.GetByCode(ctx, cleanCode)
		if err != nil {
			return nil, err
		}
		_ = u.cache.Set(ctx, cacheKey, reg, u.defaultTTL)
		return reg, nil
	})

	if err != nil {
		return nil, err
	}
	return val.(*domain.Region), nil
}

func (u *RegionUsecase) GetProvinces(ctx context.Context, filter domain.FilterParams) ([]domain.Region, int64, error) {
	cleanSearch := sanitizer.SanitizeLikePattern(filter.Search)
	orderBy := filter.OrderBy
	if orderBy == "" {
		orderBy = "code"
	}
	orderDir := filter.OrderDir
	if orderDir == "" {
		orderDir = "ASC"
	}

	cacheKey := u.cache.KeyProvincesList(cleanSearch, filter.Page, filter.Limit, orderBy, orderDir)

	var cachedResult PaginatedResult
	if found, err := u.cache.Get(ctx, cacheKey, &cachedResult); err == nil && found {
		return cachedResult.Items, cachedResult.Total, nil
	}

	val, err, _ := u.sfGroup.Do(cacheKey, func() (interface{}, error) {
		items, total, err := u.repo.GetListByLevel(ctx, domain.LevelProvince, domain.FilterParams{
			Search:   cleanSearch,
			Page:     filter.Page,
			Limit:    filter.Limit,
			OrderBy:  orderBy,
			OrderDir: orderDir,
		})
		if err != nil {
			return nil, err
		}
		res := PaginatedResult{Items: items, Total: total}
		_ = u.cache.Set(ctx, cacheKey, res, u.defaultTTL)
		return res, nil
	})

	if err != nil {
		return nil, 0, err
	}
	res := val.(PaginatedResult)
	return res.Items, res.Total, nil
}

func (u *RegionUsecase) GetChildrenByParent(ctx context.Context, parentCode string, targetLevel int16, filter domain.FilterParams) ([]domain.Region, int64, error) {
	cleanParent := strings.TrimSpace(parentCode)
	if cleanParent == "" {
		return nil, 0, domain.ErrParentNotFound
	}

	cleanSearch := sanitizer.SanitizeLikePattern(filter.Search)
	orderBy := filter.OrderBy
	if orderBy == "" {
		orderBy = "code"
	}
	orderDir := filter.OrderDir
	if orderDir == "" {
		orderDir = "ASC"
	}

	cacheKey := u.cache.KeyChildrenList(cleanParent, targetLevel, cleanSearch, filter.Page, filter.Limit, orderBy, orderDir)

	var cachedResult PaginatedResult
	if found, err := u.cache.Get(ctx, cacheKey, &cachedResult); err == nil && found {
		return cachedResult.Items, cachedResult.Total, nil
	}

	val, err, _ := u.sfGroup.Do(cacheKey, func() (interface{}, error) {
		// Validasi keberadaan parent aktif
		parent, err := u.repo.GetByCode(ctx, cleanParent)
		if err != nil {
			return nil, err
		}
		if parent.Level >= targetLevel {
			return nil, domain.ErrInvalidLevel
		}

		items, total, err := u.repo.GetChildrenByPrefix(ctx, cleanParent, targetLevel, domain.FilterParams{
			Search:   cleanSearch,
			Page:     filter.Page,
			Limit:    filter.Limit,
			OrderBy:  orderBy,
			OrderDir: orderDir,
		})
		if err != nil {
			return nil, err
		}
		res := PaginatedResult{Items: items, Total: total}
		_ = u.cache.Set(ctx, cacheKey, res, u.defaultTTL)
		return res, nil
	})

	if err != nil {
		return nil, 0, err
	}
	res := val.(PaginatedResult)
	return res.Items, res.Total, nil
}

func (u *RegionUsecase) GetPostalHierarchy(ctx context.Context, postalCode string) (*domain.PostalHierarchy, error) {
	cleanPostal := strings.TrimSpace(postalCode)
	if cleanPostal == "" {
		return nil, domain.ErrRegionNotFound
	}

	cacheKey := u.cache.KeyPostalLookup(cleanPostal)

	var cachedHierarchy domain.PostalHierarchy
	if found, err := u.cache.Get(ctx, cacheKey, &cachedHierarchy); err == nil && found {
		return &cachedHierarchy, nil
	}

	val, err, _ := u.sfGroup.Do(cacheKey, func() (interface{}, error) {
		hierarchy, err := u.repo.GetByPostalCode(ctx, cleanPostal)
		if err != nil {
			return nil, err
		}
		_ = u.cache.Set(ctx, cacheKey, hierarchy, u.defaultTTL)
		return hierarchy, nil
	})

	if err != nil {
		return nil, err
	}
	return val.(*domain.PostalHierarchy), nil
}

func (u *RegionUsecase) GetPostalCodeByVillageCode(ctx context.Context, villageCode string) (*domain.VillagePostalCodeResponse, error) {
	cleanVillage := strings.TrimSpace(villageCode)
	if cleanVillage == "" {
		return nil, domain.ErrRegionNotFound
	}

	cacheKey := fmt.Sprintf("geo:village:postal:%s", cleanVillage)
	var cachedResp domain.VillagePostalCodeResponse
	if found, err := u.cache.Get(ctx, cacheKey, &cachedResp); err == nil && found {
		return &cachedResp, nil
	}

	val, err, _ := u.sfGroup.Do(cacheKey, func() (interface{}, error) {
		resp, err := u.repo.GetPostalCodeByVillageCode(ctx, cleanVillage)
		if err != nil {
			return nil, err
		}
		_ = u.cache.Set(ctx, cacheKey, resp, u.defaultTTL)
		return resp, nil
	})

	if err != nil {
		return nil, err
	}

	return val.(*domain.VillagePostalCodeResponse), nil
}

func (u *RegionUsecase) UpdatePostalCodeByVillageCode(ctx context.Context, villageCode, postalCode string) error {
	cleanVillage := strings.TrimSpace(villageCode)
	cleanPostal := strings.TrimSpace(postalCode)

	if cleanVillage == "" {
		return domain.ErrRegionNotFound
	}

	// Validasi kode pos: 5 digit angka
	if len(cleanPostal) != 5 {
		return fmt.Errorf("%w: kode pos harus 5 digit numerik", domain.ErrInvalidLevel)
	}
	for _, c := range cleanPostal {
		if c < '0' || c > '9' {
			return fmt.Errorf("%w: kode pos harus berupa digit angka", domain.ErrInvalidLevel)
		}
	}

	// Update di database
	if err := u.repo.UpdatePostalCodeByVillageCode(ctx, cleanVillage, cleanPostal); err != nil {
		return err
	}

	// Invalidate cache: detail kelurahan dan spesifik kode pos kelurahan
	villageKey := u.cache.KeyRegionDetail(cleanVillage)
	villagePostalKey := fmt.Sprintf("geo:village:postal:%s", cleanVillage)
	_ = u.cache.Delete(ctx, villageKey, villagePostalKey)

	// Invalidate parent children list (kecamatan)
	parts := strings.Split(cleanVillage, ".")
	if len(parts) >= 3 {
		dummyVillage := &domain.Region{
			Code:  cleanVillage,
			Level: domain.LevelVillage,
		}
		_ = u.cache.InvalidateRegion(ctx, dummyVillage)
	}

	return nil
}

func (u *RegionUsecase) CreateProvince(ctx context.Context, code, name, regType string) (*domain.Region, error) {
	cleanCode := strings.TrimSpace(code)
	cleanName := strings.ToUpper(strings.TrimSpace(name))
	cleanType := strings.ToUpper(strings.TrimSpace(regType))
	if cleanType == "" {
		cleanType = "PROVINSI"
	}

	if strings.Contains(cleanCode, ".") {
		return nil, fmt.Errorf("province code cannot contain dot: %w", domain.ErrInvalidLevel)
	}

	newRegion := &domain.Region{
		Code:  cleanCode,
		Name:  cleanName,
		Level: domain.LevelProvince,
		Type:  &cleanType,
	}

	if err := u.repo.Create(ctx, newRegion); err != nil {
		return nil, err
	}

	_ = u.cache.InvalidateRegion(ctx, newRegion)
	return newRegion, nil
}

func (u *RegionUsecase) CreateChildRegion(ctx context.Context, parentCode, code, name, regType string, targetLevel int16) (*domain.Region, error) {
	cleanParent := strings.TrimSpace(parentCode)
	cleanCode := strings.TrimSpace(code)
	cleanName := strings.ToUpper(strings.TrimSpace(name))
	cleanType := strings.ToUpper(strings.TrimSpace(regType))

	// 1. Validasi Prefix: Code anak WAJIB berawalan parentCode + "."
	expectedPrefix := cleanParent + "."
	if !strings.HasPrefix(cleanCode, expectedPrefix) {
		return nil, fmt.Errorf("%w: expected '%s' as prefix for '%s'", domain.ErrInvalidChildPrefix, expectedPrefix, cleanCode)
	}

	// 2. Validasi Keberadaan Parent di Database
	parent, err := u.repo.GetByCode(ctx, cleanParent)
	if err != nil {
		return nil, domain.ErrParentNotFound
	}
	if parent.Level != targetLevel-1 {
		return nil, fmt.Errorf("%w: parent level %d cannot have child level %d", domain.ErrInvalidLevel, parent.Level, targetLevel)
	}

	newRegion := &domain.Region{
		Code:  cleanCode,
		Name:  cleanName,
		Level: targetLevel,
		Type:  &cleanType,
	}

	if err := u.repo.Create(ctx, newRegion); err != nil {
		return nil, err
	}

	_ = u.cache.InvalidateRegion(ctx, newRegion)
	return newRegion, nil
}

func (u *RegionUsecase) CreatePostalCode(ctx context.Context, villageCode, postalCode string) (*domain.Region, error) {
	cleanVillage := strings.TrimSpace(villageCode)
	cleanPostal := strings.TrimSpace(postalCode)

	// Validasi parent kelurahan (Level 4)
	village, err := u.repo.GetByCode(ctx, cleanVillage)
	if err != nil {
		return nil, domain.ErrParentNotFound
	}
	if village.Level != domain.LevelVillage {
		return nil, fmt.Errorf("%w: postal code can only be created under village level 4", domain.ErrInvalidLevel)
	}

	fullPostalCode := fmt.Sprintf("%s.%s", cleanVillage, cleanPostal)
	postalType := "KODE_POS"

	newRegion := &domain.Region{
		Code:  fullPostalCode,
		Name:  cleanPostal,
		Level: domain.LevelPostalCode,
		Type:  &postalType,
	}

	if err := u.repo.Create(ctx, newRegion); err != nil {
		return nil, err
	}

	_ = u.cache.InvalidateRegion(ctx, newRegion)
	return newRegion, nil
}

func (u *RegionUsecase) UpdateRegion(ctx context.Context, code, name, regType string) (*domain.Region, error) {
	cleanCode := strings.TrimSpace(code)
	cleanName := strings.ToUpper(strings.TrimSpace(name))
	cleanType := strings.ToUpper(strings.TrimSpace(regType))

	existing, err := u.repo.GetByCode(ctx, cleanCode)
	if err != nil {
		return nil, err
	}

	existing.Name = cleanName
	if cleanType != "" {
		existing.Type = &cleanType
	}

	if err := u.repo.Update(ctx, existing); err != nil {
		return nil, err
	}

	_ = u.cache.InvalidateRegion(ctx, existing)
	return existing, nil
}

func (u *RegionUsecase) DeleteRegion(ctx context.Context, code string) error {
	cleanCode := strings.TrimSpace(code)

	existing, err := u.repo.GetByCode(ctx, cleanCode)
	if err != nil {
		return err
	}

	// Cek apakah memiliki child aktif
	hasChild, err := u.repo.HasActiveChildren(ctx, cleanCode)
	if err != nil {
		return err
	}
	if hasChild {
		return domain.ErrCannotDeleteWithActiveChildren
	}

	if err := u.repo.Delete(ctx, cleanCode); err != nil {
		return err
	}

	_ = u.cache.InvalidateRegion(ctx, existing)
	return nil
}
