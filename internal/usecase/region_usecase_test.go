package usecase_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"be_region_indonesia/internal/domain"
	"be_region_indonesia/internal/usecase"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockRegionRepository
type MockRegionRepository struct {
	mock.Mock
}

func (m *MockRegionRepository) GetByCode(ctx context.Context, code string) (*domain.Region, error) {
	args := m.Called(ctx, code)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Region), args.Error(1)
}

func (m *MockRegionRepository) GetListByLevel(ctx context.Context, level int16, filter domain.FilterParams) ([]domain.Region, int64, error) {
	args := m.Called(ctx, level, filter)
	return args.Get(0).([]domain.Region), args.Get(1).(int64), args.Error(2)
}

func (m *MockRegionRepository) GetChildrenByPrefix(ctx context.Context, parentCode string, targetLevel int16, filter domain.FilterParams) ([]domain.Region, int64, error) {
	args := m.Called(ctx, parentCode, targetLevel, filter)
	return args.Get(0).([]domain.Region), args.Get(1).(int64), args.Error(2)
}

func (m *MockRegionRepository) GetByPostalCode(ctx context.Context, postalCode string) (*domain.PostalHierarchy, error) {
	args := m.Called(ctx, postalCode)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.PostalHierarchy), args.Error(1)
}

func (m *MockRegionRepository) Create(ctx context.Context, region *domain.Region) error {
	args := m.Called(ctx, region)
	return args.Error(0)
}

func (m *MockRegionRepository) Update(ctx context.Context, region *domain.Region) error {
	args := m.Called(ctx, region)
	return args.Error(0)
}

func (m *MockRegionRepository) Delete(ctx context.Context, code string) error {
	args := m.Called(ctx, code)
	return args.Error(0)
}

func (m *MockRegionRepository) HasActiveChildren(ctx context.Context, parentCode string) (bool, error) {
	args := m.Called(ctx, parentCode)
	return args.Bool(0), args.Error(1)
}

func (m *MockRegionRepository) GetPostalCodeByVillageCode(ctx context.Context, villageCode string) (*domain.VillagePostalCodeResponse, error) {
	args := m.Called(ctx, villageCode)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.VillagePostalCodeResponse), args.Error(1)
}

func (m *MockRegionRepository) UpdatePostalCodeByVillageCode(ctx context.Context, villageCode, postalCode string) error {
	args := m.Called(ctx, villageCode, postalCode)
	return args.Error(0)
}

func (m *MockRegionRepository) BatchUpdatePostalCodes(ctx context.Context, codes []string, postalCodes []string) (int64, error) {
	args := m.Called(ctx, codes, postalCodes)
	return args.Get(0).(int64), args.Error(1)
}

// MockRegionCacheRepository
type MockRegionCacheRepository struct {
	mock.Mock
}

func (m *MockRegionCacheRepository) Get(ctx context.Context, key string, target interface{}) (bool, error) {
	args := m.Called(ctx, key, target)
	return args.Bool(0), args.Error(1)
}

func (m *MockRegionCacheRepository) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	args := m.Called(ctx, key, value, ttl)
	return args.Error(0)
}

func (m *MockRegionCacheRepository) Delete(ctx context.Context, keys ...string) error {
	args := m.Called(ctx, keys)
	return args.Error(0)
}

func (m *MockRegionCacheRepository) InvalidateRegion(ctx context.Context, region *domain.Region) error {
	args := m.Called(ctx, region)
	return args.Error(0)
}

func (m *MockRegionCacheRepository) KeyProvincesList(search string, page, limit int, orderBy, orderDir string) string {
	return fmt.Sprintf("geo:provinces:list:%s:%d:%d:%s:%s", search, page, limit, orderBy, orderDir)
}

func (m *MockRegionCacheRepository) KeyRegionDetail(code string) string {
	return fmt.Sprintf("geo:detail:%s", code)
}

func (m *MockRegionCacheRepository) KeyChildrenList(parentCode string, targetLevel int16, search string, page, limit int, orderBy, orderDir string) string {
	return fmt.Sprintf("geo:children:%s:%d:%s:%d:%d:%s:%s", parentCode, targetLevel, search, page, limit, orderBy, orderDir)
}

func (m *MockRegionCacheRepository) KeyPostalLookup(postalCode string) string {
	return fmt.Sprintf("geo:postal:lookup:%s", postalCode)
}

// ==========================================
// TEST CASES
// ==========================================

func TestRegionUsecase_GetByCode_CacheHitAndMiss(t *testing.T) {
	mockRepo := new(MockRegionRepository)
	mockCache := new(MockRegionCacheRepository)
	uc := usecase.NewRegionUsecase(mockRepo, mockCache, 168)

	ctx := context.Background()
	detailKey := "geo:detail:11"
	sampleRegion := &domain.Region{Code: "11", Name: "ACEH", Level: 1}

	// Case 1: Cache Miss -> Repo Called -> Cache Set
	mockCache.On("Get", ctx, detailKey, mock.Anything).Return(false, nil).Once()
	mockRepo.On("GetByCode", ctx, "11").Return(sampleRegion, nil).Once()
	mockCache.On("Set", ctx, detailKey, sampleRegion, 168*time.Hour).Return(nil).Once()

	res, err := uc.GetByCode(ctx, "11")
	require.NoError(t, err)
	assert.Equal(t, "11", res.Code)

	// Case 2: Empty code -> ErrRegionNotFound
	_, err = uc.GetByCode(ctx, "  ")
	assert.ErrorIs(t, err, domain.ErrRegionNotFound)

	mockRepo.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}

func TestRegionUsecase_GetProvinces(t *testing.T) {
	mockRepo := new(MockRegionRepository)
	mockCache := new(MockRegionCacheRepository)
	uc := usecase.NewRegionUsecase(mockRepo, mockCache, 168)

	ctx := context.Background()
	cacheKey := "geo:provinces:list::1:20:code:ASC"
	items := []domain.Region{{Code: "11", Name: "ACEH", Level: 1}}

	mockCache.On("Get", ctx, cacheKey, mock.Anything).Return(false, nil).Once()
	mockRepo.On("GetListByLevel", ctx, domain.LevelProvince, domain.FilterParams{Page: 1, Limit: 20, OrderBy: "code", OrderDir: "ASC"}).
		Return(items, int64(1), nil).Once()
	mockCache.On("Set", ctx, cacheKey, mock.Anything, 168*time.Hour).Return(nil).Once()

	result, total, err := uc.GetProvinces(ctx, domain.FilterParams{Page: 1, Limit: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, result, 1)

	mockRepo.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}

func TestRegionUsecase_GetChildrenByParent(t *testing.T) {
	mockRepo := new(MockRegionRepository)
	mockCache := new(MockRegionCacheRepository)
	uc := usecase.NewRegionUsecase(mockRepo, mockCache, 168)

	ctx := context.Background()
	parent := &domain.Region{Code: "11", Name: "ACEH", Level: 1}
	children := []domain.Region{{Code: "11.01", Name: "KABUPATEN ACEH SELATAN", Level: 2}}
	cacheKey := "geo:children:11:2::1:20:code:ASC"

	mockCache.On("Get", ctx, cacheKey, mock.Anything).Return(false, nil).Once()
	mockRepo.On("GetByCode", ctx, "11").Return(parent, nil).Once()
	mockRepo.On("GetChildrenByPrefix", ctx, "11", domain.LevelRegency, domain.FilterParams{Page: 1, Limit: 20, OrderBy: "code", OrderDir: "ASC"}).
		Return(children, int64(1), nil).Once()
	mockCache.On("Set", ctx, cacheKey, mock.Anything, 168*time.Hour).Return(nil).Once()

	res, total, err := uc.GetChildrenByParent(ctx, "11", domain.LevelRegency, domain.FilterParams{Page: 1, Limit: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, res, 1)

	// Validation: Empty parent
	_, _, err = uc.GetChildrenByParent(ctx, "", 2, domain.FilterParams{})
	assert.ErrorIs(t, err, domain.ErrParentNotFound)

	mockRepo.AssertExpectations(t)
}

func TestRegionUsecase_CreateProvince(t *testing.T) {
	mockRepo := new(MockRegionRepository)
	mockCache := new(MockRegionCacheRepository)
	uc := usecase.NewRegionUsecase(mockRepo, mockCache, 168)

	ctx := context.Background()

	// 1. Success Create Province
	mockRepo.On("Create", ctx, mock.MatchedBy(func(r *domain.Region) bool {
		return r.Code == "99" && r.Name == "PROVINSI BARU" && r.Level == 1
	})).Return(nil).Once()
	mockCache.On("InvalidateRegion", ctx, mock.Anything).Return(nil).Once()

	res, err := uc.CreateProvince(ctx, "99", "Provinsi Baru", "PROVINSI")
	require.NoError(t, err)
	assert.Equal(t, "99", res.Code)
	assert.Equal(t, "PROVINSI BARU", res.Name)

	// 2. Reject dot in province code
	_, err = uc.CreateProvince(ctx, "99.01", "Provinsi Gagal", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot contain dot")
}

func TestRegionUsecase_CreateChildRegion_PrefixValidation(t *testing.T) {
	mockRepo := new(MockRegionRepository)
	mockCache := new(MockRegionCacheRepository)
	uc := usecase.NewRegionUsecase(mockRepo, mockCache, 168)

	ctx := context.Background()

	// Skenario 1: Prefix Tidak Cocok (Ditolak Langsung)
	_, err := uc.CreateChildRegion(ctx, "11", "12.01", "KABUPATEN GAGAL", "KABUPATEN", domain.LevelRegency)
	assert.ErrorIs(t, err, domain.ErrInvalidChildPrefix)

	// Skenario 2: Parent Tidak Ditemukan
	mockRepo.On("GetByCode", ctx, "11").Return(nil, domain.ErrRegionNotFound).Once()
	_, err = uc.CreateChildRegion(ctx, "11", "11.01", "KABUPATEN GAGAL", "KABUPATEN", domain.LevelRegency)
	assert.ErrorIs(t, err, domain.ErrParentNotFound)

	// Skenario 3: Level Tidak Sesuai (Parent level 1 tapi target level 3)
	parentProv := &domain.Region{Code: "11", Level: domain.LevelProvince}
	mockRepo.On("GetByCode", ctx, "11").Return(parentProv, nil).Once()
	_, err = uc.CreateChildRegion(ctx, "11", "11.01", "KECAMATAN GAGAL", "KECAMATAN", domain.LevelDistrict)
	assert.ErrorIs(t, err, domain.ErrInvalidLevel)

	// Skenario 4: Sukses
	mockRepo.On("GetByCode", ctx, "11").Return(parentProv, nil).Once()
	mockRepo.On("Create", ctx, mock.MatchedBy(func(r *domain.Region) bool {
		return r.Code == "11.01" && r.Level == domain.LevelRegency
	})).Return(nil).Once()
	mockCache.On("InvalidateRegion", ctx, mock.Anything).Return(nil).Once()

	res, err := uc.CreateChildRegion(ctx, "11", "11.01", "Kabupaten Sukses", "KABUPATEN", domain.LevelRegency)
	require.NoError(t, err)
	assert.Equal(t, "11.01", res.Code)

	mockRepo.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}

func TestRegionUsecase_CreatePostalCode(t *testing.T) {
	mockRepo := new(MockRegionRepository)
	mockCache := new(MockRegionCacheRepository)
	uc := usecase.NewRegionUsecase(mockRepo, mockCache, 168)

	ctx := context.Background()
	village := &domain.Region{Code: "11.01.01.2001", Level: domain.LevelVillage}

	// 1. Success Create Postal Code
	mockRepo.On("GetByCode", ctx, "11.01.01.2001").Return(village, nil).Once()
	mockRepo.On("Create", ctx, mock.MatchedBy(func(r *domain.Region) bool {
		return r.Code == "11.01.01.2001.24264" && r.Name == "24264" && r.Level == domain.LevelPostalCode
	})).Return(nil).Once()
	mockCache.On("InvalidateRegion", ctx, mock.Anything).Return(nil).Once()

	res, err := uc.CreatePostalCode(ctx, "11.01.01.2001", "24264")
	require.NoError(t, err)
	assert.Equal(t, "11.01.01.2001.24264", res.Code)
	assert.Equal(t, "24264", res.Name)

	// 2. Reject if parent is not LevelVillage
	nonVillage := &domain.Region{Code: "11.01.01", Level: domain.LevelDistrict}
	mockRepo.On("GetByCode", ctx, "11.01.01").Return(nonVillage, nil).Once()
	_, err = uc.CreatePostalCode(ctx, "11.01.01", "24264")
	assert.ErrorIs(t, err, domain.ErrInvalidLevel)
}

func TestRegionUsecase_DeleteRegion_Protection(t *testing.T) {
	mockRepo := new(MockRegionRepository)
	mockCache := new(MockRegionCacheRepository)
	uc := usecase.NewRegionUsecase(mockRepo, mockCache, 168)

	ctx := context.Background()
	existing := &domain.Region{Code: "11", Level: 1}

	// Skenario 1: Ditolak karena memiliki child aktif
	mockRepo.On("GetByCode", ctx, "11").Return(existing, nil).Once()
	mockRepo.On("HasActiveChildren", ctx, "11").Return(true, nil).Once()

	err := uc.DeleteRegion(ctx, "11")
	assert.ErrorIs(t, err, domain.ErrCannotDeleteWithActiveChildren)

	// Skenario 2: Berhasil dihapus jika tidak ada child aktif
	mockRepo.On("GetByCode", ctx, "11").Return(existing, nil).Once()
	mockRepo.On("HasActiveChildren", ctx, "11").Return(false, nil).Once()
	mockRepo.On("Delete", ctx, "11").Return(nil).Once()
	mockCache.On("InvalidateRegion", ctx, existing).Return(nil).Once()

	err = uc.DeleteRegion(ctx, "11")
	require.NoError(t, err)

	mockRepo.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}

func TestRegionUsecase_UpdateRegion(t *testing.T) {
	mockRepo := new(MockRegionRepository)
	mockCache := new(MockRegionCacheRepository)
	uc := usecase.NewRegionUsecase(mockRepo, mockCache, 168)

	ctx := context.Background()
	existing := &domain.Region{Code: "11", Name: "LAMA", Level: 1}

	mockRepo.On("GetByCode", ctx, "11").Return(existing, nil).Once()
	mockRepo.On("Update", ctx, mock.MatchedBy(func(r *domain.Region) bool {
		return r.Name == "BARU DIUBAH"
	})).Return(nil).Once()
	mockCache.On("InvalidateRegion", ctx, mock.Anything).Return(nil).Once()

	updated, err := uc.UpdateRegion(ctx, "11", "Baru Diubah", "PROVINSI")
	require.NoError(t, err)
	assert.Equal(t, "BARU DIUBAH", updated.Name)
}

func TestRegionUsecase_GetPostalHierarchy(t *testing.T) {
	mockRepo := new(MockRegionRepository)
	mockCache := new(MockRegionCacheRepository)
	uc := usecase.NewRegionUsecase(mockRepo, mockCache, 168)

	ctx := context.Background()
	cacheKey := "geo:postal:lookup:24264"
	hierarchy := &domain.PostalHierarchy{PostalCode: "24264"}

	mockCache.On("Get", ctx, cacheKey, mock.Anything).Return(false, nil).Once()
	mockRepo.On("GetByPostalCode", ctx, "24264").Return(hierarchy, nil).Once()
	mockCache.On("Set", ctx, cacheKey, hierarchy, 168*time.Hour).Return(nil).Once()

	res, err := uc.GetPostalHierarchy(ctx, "24264")
	require.NoError(t, err)
	assert.Equal(t, "24264", res.PostalCode)

	// Empty postal code
	_, err = uc.GetPostalHierarchy(ctx, "  ")
	assert.ErrorIs(t, err, domain.ErrRegionNotFound)
}


func TestRegionUsecase_ErrorBranches(t *testing.T) {
	mockRepo := new(MockRegionRepository)
	mockCache := new(MockRegionCacheRepository)
	uc := usecase.NewRegionUsecase(mockRepo, mockCache, 0) // Test defaultTTLHours <= 0

	ctx := context.Background()

	// 1. GetByCode repo error
	mockCache.On("Get", ctx, "geo:detail:11", mock.Anything).Return(false, nil).Once()
	mockRepo.On("GetByCode", ctx, "11").Return(nil, domain.ErrRegionNotFound).Once()
	_, err := uc.GetByCode(ctx, "11")
	assert.ErrorIs(t, err, domain.ErrRegionNotFound)

	// 2. GetProvinces repo error
	mockCache.On("Get", ctx, "geo:provinces:list::1:20:code:ASC", mock.Anything).Return(false, nil).Once()
	mockRepo.On("GetListByLevel", ctx, domain.LevelProvince, domain.FilterParams{Page: 1, Limit: 20, OrderBy: "code", OrderDir: "ASC"}).
		Return([]domain.Region{}, int64(0), domain.ErrRegionNotFound).Once()
	_, _, err = uc.GetProvinces(ctx, domain.FilterParams{Page: 1, Limit: 20})
	assert.ErrorIs(t, err, domain.ErrRegionNotFound)

	// 3. GetPostalHierarchy repo error
	mockCache.On("Get", ctx, "geo:postal:lookup:24264", mock.Anything).Return(false, nil).Once()
	mockRepo.On("GetByPostalCode", ctx, "24264").Return(nil, domain.ErrRegionNotFound).Once()
	_, err = uc.GetPostalHierarchy(ctx, "24264")
	assert.ErrorIs(t, err, domain.ErrRegionNotFound)

	// 4. UpdateRegion repo error
	mockRepo.On("GetByCode", ctx, "99").Return(nil, domain.ErrRegionNotFound).Once()
	_, err = uc.UpdateRegion(ctx, "99", "Baru", "PROVINSI")
	assert.ErrorIs(t, err, domain.ErrRegionNotFound)

	// 5. DeleteRegion repo error on get
	mockRepo.On("GetByCode", ctx, "99").Return(nil, domain.ErrRegionNotFound).Once()
	err = uc.DeleteRegion(ctx, "99")
	assert.ErrorIs(t, err, domain.ErrRegionNotFound)

	// 6. DeleteRegion repo error on check children
	existing := &domain.Region{Code: "11", Level: 1}
	mockRepo.On("GetByCode", ctx, "11").Return(existing, nil).Once()
	mockRepo.On("HasActiveChildren", ctx, "11").Return(false, domain.ErrRegionNotFound).Once()
	err = uc.DeleteRegion(ctx, "11")
	assert.ErrorIs(t, err, domain.ErrRegionNotFound)
}

func TestRegionUsecase_GetPostalCodeByVillageCode(t *testing.T) {
	mockRepo := new(MockRegionRepository)
	mockCache := new(MockRegionCacheRepository)
	uc := usecase.NewRegionUsecase(mockRepo, mockCache, 168)
	ctx := context.Background()

	t.Run("invalid village code empty", func(t *testing.T) {
		res, err := uc.GetPostalCodeByVillageCode(ctx, "   ")
		assert.ErrorIs(t, err, domain.ErrRegionNotFound)
		assert.Nil(t, res)
	})

	t.Run("cache hit", func(t *testing.T) {
		villageCode := "11.01.01.2001"
		postalCode := "23773"
		cachedResp := &domain.VillagePostalCodeResponse{
			VillageCode: villageCode,
			VillageName: "BAKONGAN",
			PostalCode:  &postalCode,
		}

		mockCache.On("Get", ctx, "geo:village:postal:"+villageCode, mock.AnythingOfType("*domain.VillagePostalCodeResponse")).
			Run(func(args mock.Arguments) {
				arg := args.Get(2).(*domain.VillagePostalCodeResponse)
				*arg = *cachedResp
			}).
			Return(true, nil).Once()

		res, err := uc.GetPostalCodeByVillageCode(ctx, villageCode)
		require.NoError(t, err)
		assert.Equal(t, cachedResp, res)
	})

	t.Run("cache miss - repo success", func(t *testing.T) {
		villageCode := "11.01.01.2002"
		postalCode := "23773"
		repoResp := &domain.VillagePostalCodeResponse{
			VillageCode: villageCode,
			VillageName: "PADANG BARU",
			PostalCode:  &postalCode,
		}

		mockCache.On("Get", ctx, "geo:village:postal:"+villageCode, mock.AnythingOfType("*domain.VillagePostalCodeResponse")).
			Return(false, nil).Once()
		mockRepo.On("GetPostalCodeByVillageCode", ctx, villageCode).
			Return(repoResp, nil).Once()
		mockCache.On("Set", ctx, "geo:village:postal:"+villageCode, repoResp, 168*time.Hour).
			Return(nil).Once()

		res, err := uc.GetPostalCodeByVillageCode(ctx, villageCode)
		require.NoError(t, err)
		assert.Equal(t, repoResp, res)
	})
}

func TestRegionUsecase_UpdatePostalCodeByVillageCode(t *testing.T) {
	mockRepo := new(MockRegionRepository)
	mockCache := new(MockRegionCacheRepository)
	uc := usecase.NewRegionUsecase(mockRepo, mockCache, 168)
	ctx := context.Background()

	t.Run("invalid postal code format", func(t *testing.T) {
		err := uc.UpdatePostalCodeByVillageCode(ctx, "11.01.01.2001", "123")
		assert.ErrorIs(t, err, domain.ErrInvalidLevel)
	})

	t.Run("invalid village code empty", func(t *testing.T) {
		err := uc.UpdatePostalCodeByVillageCode(ctx, "  ", "12345")
		assert.ErrorIs(t, err, domain.ErrRegionNotFound)
	})

	t.Run("success update and cache invalidation", func(t *testing.T) {
		villageCode := "11.01.01.2001"
		postalCode := "23773"

		mockRepo.On("UpdatePostalCodeByVillageCode", ctx, villageCode, postalCode).
			Return(nil).Once()
		mockCache.On("KeyRegionDetail", villageCode).
			Return("geo:detail:" + villageCode).Once()
		mockCache.On("Delete", ctx, []string{"geo:detail:" + villageCode, "geo:village:postal:" + villageCode}).
			Return(nil).Once()
		mockCache.On("InvalidateRegion", ctx, mock.AnythingOfType("*domain.Region")).
			Return(nil).Once()

		err := uc.UpdatePostalCodeByVillageCode(ctx, villageCode, postalCode)
		require.NoError(t, err)
	})
}
