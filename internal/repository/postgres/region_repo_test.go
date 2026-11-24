package postgres_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"be_region_indonesia/config"
	"be_region_indonesia/internal/domain"
	"be_region_indonesia/internal/repository/postgres"
	"be_region_indonesia/pkg/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRepo(t *testing.T) (*postgres.RegionRepository, *database.PostgresPool) {
	rootEnv := filepath.Join("..", "..", "..", ".env")
	cfg, err := config.LoadConfig(rootEnv)
	require.NoError(t, err)

	pool, err := database.NewPostgresConnection(cfg)
	require.NoError(t, err)

	repo := postgres.NewRegionRepository(pool.DB)
	return repo, pool
}

func TestRegionRepository_GetByCode(t *testing.T) {
	repo, pool := setupTestRepo(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Kode 11 adalah Provinsi Aceh dari seeder
	reg, err := repo.GetByCode(ctx, "11")
	require.NoError(t, err)
	assert.NotNil(t, reg)
	assert.Equal(t, "11", reg.Code)
	assert.Equal(t, domain.LevelProvince, reg.Level)
	assert.Equal(t, "ACEH", reg.Name)

	// Test non-existing code
	_, err = repo.GetByCode(ctx, "99999999")
	assert.ErrorIs(t, err, domain.ErrRegionNotFound)
}

func TestRegionRepository_GetListByLevel(t *testing.T) {
	repo, pool := setupTestRepo(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Test mengambil list 38 provinsi
	list, total, err := repo.GetListByLevel(ctx, domain.LevelProvince, domain.FilterParams{Page: 1, Limit: 50})
	require.NoError(t, err)
	assert.Equal(t, int64(38), total)
	assert.Len(t, list, 38)

	// Test search filter
	list, total, err = repo.GetListByLevel(ctx, domain.LevelProvince, domain.FilterParams{Search: "JAKARTA", Page: 1, Limit: 10})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1))
	assert.Contains(t, list[0].Name, "JAKARTA")
}

func TestRegionRepository_GetChildrenByPrefix(t *testing.T) {
	repo, pool := setupTestRepo(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Ambil Kabupaten/Kota di bawah Provinsi Aceh (code: 11)
	children, total, err := repo.GetChildrenByPrefix(ctx, "11", domain.LevelRegency, domain.FilterParams{Page: 1, Limit: 50})
	require.NoError(t, err)
	assert.Equal(t, int64(23), total)
	assert.Len(t, children, 23)

	for _, child := range children {
		assert.Equal(t, domain.LevelRegency, child.Level)
		assert.True(t, len(child.Code) > 3 && child.Code[:3] == "11.")
	}
}

func TestRegionRepository_CRUD_Lifecycle(t *testing.T) {
	repo, pool := setupTestRepo(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	testProvCode := "88"
	testType := "PROVINSI"

	// Cleanup if exists
	_ = repo.Delete(ctx, testProvCode)

	// 1. Create
	newProv := &domain.Region{
		Code:  testProvCode,
		Name:  "PROVINSI TEST BARU",
		Level: domain.LevelProvince,
		Type:  &testType,
	}
	err := repo.Create(ctx, newProv)
	require.NoError(t, err)
	assert.Greater(t, newProv.ID, int64(0))

	// 1b. Duplicate code check
	err = repo.Create(ctx, newProv)
	assert.ErrorIs(t, err, domain.ErrRegionCodeAlreadyExists)

	// 2. HasActiveChildren should be false initially
	hasChild, err := repo.HasActiveChildren(ctx, testProvCode)
	require.NoError(t, err)
	assert.False(t, hasChild)

	// Create child kabupaten
	testRegCode := "88.01"
	regType := "KABUPATEN"
	newReg := &domain.Region{
		Code:  testRegCode,
		Name:  "KABUPATEN TEST ANAK",
		Level: domain.LevelRegency,
		Type:  &regType,
	}
	err = repo.Create(ctx, newReg)
	require.NoError(t, err)

	// 3. HasActiveChildren should now be true
	hasChild, err = repo.HasActiveChildren(ctx, testProvCode)
	require.NoError(t, err)
	assert.True(t, hasChild)

	// 4. Update
	updatedName := "PROVINSI TEST DIUBAH"
	newProv.Name = updatedName
	err = repo.Update(ctx, newProv)
	require.NoError(t, err)

	fetched, err := repo.GetByCode(ctx, testProvCode)
	require.NoError(t, err)
	assert.Equal(t, updatedName, fetched.Name)

	// 5. Delete child first, then parent
	err = repo.Delete(ctx, testRegCode)
	require.NoError(t, err)

	err = repo.Delete(ctx, testProvCode)
	require.NoError(t, err)

	// Verify not found after soft-delete
	_, err = repo.GetByCode(ctx, testProvCode)
	assert.ErrorIs(t, err, domain.ErrRegionNotFound)
}

func TestRegionRepository_PostalCode(t *testing.T) {
	repo, pool := setupTestRepo(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	postalType := "KODE_POS"
	postalVal := "99887"
	testPostal := &domain.Region{
		Code:  "11.01.01.2001.99887",
		Name:  postalVal,
		Level: domain.LevelPostalCode,
		Type:  &postalType,
	}

	_ = repo.Delete(ctx, testPostal.Code)

	err := repo.Create(ctx, testPostal)
	require.NoError(t, err)
	defer func() { _ = repo.Delete(ctx, testPostal.Code) }()

	hierarchy, err := repo.GetByPostalCode(ctx, postalVal)
	require.NoError(t, err)
	assert.Equal(t, postalVal, hierarchy.PostalCode)
	assert.NotNil(t, hierarchy.Village)
	assert.Equal(t, "11.01.01.2001", hierarchy.Village.Code)
	assert.NotNil(t, hierarchy.District)
	assert.Equal(t, "11.01.01", hierarchy.District.Code)
	assert.NotNil(t, hierarchy.Regency)
	assert.Equal(t, "11.01", hierarchy.Regency.Code)
	assert.NotNil(t, hierarchy.Province)
	assert.Equal(t, "11", hierarchy.Province.Code)
}
