package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"be_region_indonesia/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockRegionRepo struct {
	mock.Mock
}

func (m *MockRegionRepo) GetByCode(ctx context.Context, code string) (*domain.Region, error) {
	args := m.Called(ctx, code)
	return args.Get(0).(*domain.Region), args.Error(1)
}
func (m *MockRegionRepo) GetListByLevel(ctx context.Context, level int16, filter domain.FilterParams) ([]domain.Region, int64, error) {
	args := m.Called(ctx, level, filter)
	return args.Get(0).([]domain.Region), args.Get(1).(int64), args.Error(2)
}
func (m *MockRegionRepo) GetChildrenByPrefix(ctx context.Context, parentCode string, targetLevel int16, filter domain.FilterParams) ([]domain.Region, int64, error) {
	args := m.Called(ctx, parentCode, targetLevel, filter)
	return args.Get(0).([]domain.Region), args.Get(1).(int64), args.Error(2)
}
func (m *MockRegionRepo) GetByPostalCode(ctx context.Context, postalCode string) (*domain.PostalHierarchy, error) {
	args := m.Called(ctx, postalCode)
	return args.Get(0).(*domain.PostalHierarchy), args.Error(1)
}
func (m *MockRegionRepo) GetPostalCodeByVillageCode(ctx context.Context, villageCode string) (*domain.VillagePostalCodeResponse, error) {
	args := m.Called(ctx, villageCode)
	return args.Get(0).(*domain.VillagePostalCodeResponse), args.Error(1)
}
func (m *MockRegionRepo) UpdatePostalCodeByVillageCode(ctx context.Context, villageCode, postalCode string) error {
	args := m.Called(ctx, villageCode, postalCode)
	return args.Error(0)
}
func (m *MockRegionRepo) BatchUpdatePostalCodes(ctx context.Context, codes []string, postalCodes []string) (int64, error) {
	args := m.Called(ctx, codes, postalCodes)
	return args.Get(0).(int64), args.Error(1)
}
func (m *MockRegionRepo) Create(ctx context.Context, region *domain.Region) error {
	args := m.Called(ctx, region)
	return args.Error(0)
}
func (m *MockRegionRepo) Update(ctx context.Context, region *domain.Region) error {
	args := m.Called(ctx, region)
	return args.Error(0)
}
func (m *MockRegionRepo) Delete(ctx context.Context, code string) error {
	args := m.Called(ctx, code)
	return args.Error(0)
}
func (m *MockRegionRepo) HasActiveChildren(ctx context.Context, parentCode string) (bool, error) {
	args := m.Called(ctx, parentCode)
	return args.Bool(0), args.Error(1)
}

func TestRunImport_ValidCSV(t *testing.T) {
	tmpDir := t.TempDir()
	csvFile := filepath.Join(tmpDir, "test_valid.csv")
	content := `code,postal_code
11.01.01.2001,23773
11.01.01.2002,23773
invalid_row_too_short
11.01.01.2003,abcde
11.01.01.2004,1234
11.01.01.2005,12345
`
	err := os.WriteFile(csvFile, []byte(content), 0644)
	require.NoError(t, err)

	mockRepo := new(MockRegionRepo)
	ctx := context.Background()

	// Hanya 3 baris valid: 2001, 2002, 2005
	mockRepo.On("BatchUpdatePostalCodes", ctx, []string{"11.01.01.2001", "11.01.01.2002", "11.01.01.2005"}, []string{"23773", "23773", "12345"}).
		Return(int64(3), nil).Once()

	stats, err := RunImport(ctx, csvFile, 10, false, mockRepo, nil)
	require.NoError(t, err)

	assert.Equal(t, 6, stats.TotalRead)
	assert.Equal(t, 3, stats.ValidRows)
	assert.Equal(t, 3, stats.InvalidFormat)
	assert.Equal(t, int64(3), stats.UpdatedRows)
	assert.Equal(t, 0, stats.SkippedRows)
}

func TestRunImport_InvalidHeader(t *testing.T) {
	tmpDir := t.TempDir()
	csvFile := filepath.Join(tmpDir, "bad_header.csv")
	content := `id,name,level
1,Aceh,1
`
	err := os.WriteFile(csvFile, []byte(content), 0644)
	require.NoError(t, err)

	mockRepo := new(MockRegionRepo)
	ctx := context.Background()

	_, err = RunImport(ctx, csvFile, 10, false, mockRepo, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "format header CSV tidak valid")
}
