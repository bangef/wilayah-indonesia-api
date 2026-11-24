package domain

import (
	"context"
	"errors"
	"time"
)

// Regional Level Hierarchy Constants
const (
	LevelProvince   int16 = 1
	LevelRegency    int16 = 2
	LevelDistrict   int16 = 3
	LevelVillage    int16 = 4
	LevelPostalCode int16 = 5
)

// Common Domain Errors
var (
	ErrRegionNotFound                 = errors.New("data wilayah tidak ditemukan")
	ErrRegionCodeAlreadyExists        = errors.New("kode wilayah sudah terdaftar dalam sistem")
	ErrParentNotFound                 = errors.New("wilayah induk tidak ditemukan")
	ErrInvalidChildPrefix             = errors.New("kode wilayah turunan harus diawali dengan kode wilayah induk sebagai prefix")
	ErrCannotDeleteWithActiveChildren = errors.New("tidak dapat menghapus wilayah yang masih memiliki entitas anak aktif")
	ErrInvalidLevel                   = errors.New("tingkat hierarki wilayah tidak valid (harus berada di rentang level 1 hingga 5)")
)

type Region struct {
	ID         int64      `json:"id"`
	Code       string     `json:"code"`
	Name       string     `json:"name"`
	Level      int16      `json:"level"`
	Type       *string    `json:"type,omitempty"`
	PostalCode *string    `json:"postal_code,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	DeletedAt  *time.Time `json:"deleted_at,omitempty"`
}

type FilterParams struct {
	Search   string
	Page     int
	Limit    int
	OrderBy  string // "code" | "name" (default: "code")
	OrderDir string // "ASC" | "DESC" (default: "ASC")
}

type VillagePostalCodeResponse struct {
	VillageCode string  `json:"village_code"`
	VillageName string  `json:"village_name"`
	PostalCode  *string `json:"postal_code"`
}

type PostalHierarchy struct {
	PostalCode string  `json:"postal_code"`
	Village    *Region `json:"village,omitempty"`
	District   *Region `json:"district,omitempty"`
	Regency    *Region `json:"regency,omitempty"`
	Province   *Region `json:"province,omitempty"`
}

// RegionRepository mendefinisikan kontrak akses data ke PostgreSQL
type RegionRepository interface {
	GetByCode(ctx context.Context, code string) (*Region, error)
	GetListByLevel(ctx context.Context, level int16, filter FilterParams) ([]Region, int64, error)
	GetChildrenByPrefix(ctx context.Context, parentCode string, targetLevel int16, filter FilterParams) ([]Region, int64, error)
	GetByPostalCode(ctx context.Context, postalCode string) (*PostalHierarchy, error)
	GetPostalCodeByVillageCode(ctx context.Context, villageCode string) (*VillagePostalCodeResponse, error)
	UpdatePostalCodeByVillageCode(ctx context.Context, villageCode, postalCode string) error
	BatchUpdatePostalCodes(ctx context.Context, codes []string, postalCodes []string) (int64, error)
	Create(ctx context.Context, region *Region) error
	Update(ctx context.Context, region *Region) error
	Delete(ctx context.Context, code string) error
	HasActiveChildren(ctx context.Context, parentCode string) (bool, error)
}

// RegionUsecase mendefinisikan seluruh business logic, validasi hierarki, dan orkestrasi caching
type RegionUsecase interface {
	GetByCode(ctx context.Context, code string) (*Region, error)
	GetProvinces(ctx context.Context, filter FilterParams) ([]Region, int64, error)
	GetChildrenByParent(ctx context.Context, parentCode string, targetLevel int16, filter FilterParams) ([]Region, int64, error)
	GetPostalHierarchy(ctx context.Context, postalCode string) (*PostalHierarchy, error)
	GetPostalCodeByVillageCode(ctx context.Context, villageCode string) (*VillagePostalCodeResponse, error)
	UpdatePostalCodeByVillageCode(ctx context.Context, villageCode, postalCode string) error

	CreateProvince(ctx context.Context, code, name, regType string) (*Region, error)
	CreateChildRegion(ctx context.Context, parentCode, code, name, regType string, targetLevel int16) (*Region, error)
	CreatePostalCode(ctx context.Context, villageCode, postalCode string) (*Region, error)

	UpdateRegion(ctx context.Context, code, name, regType string) (*Region, error)
	DeleteRegion(ctx context.Context, code string) error
}
