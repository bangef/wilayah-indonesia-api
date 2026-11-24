package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"be_region_indonesia/internal/domain"
	"github.com/lib/pq"
)

type RegionRepository struct {
	db *sql.DB
}

func NewRegionRepository(db *sql.DB) *RegionRepository {
	return &RegionRepository{db: db}
}

func (r *RegionRepository) GetByCode(ctx context.Context, code string) (*domain.Region, error) {
	query := `
		SELECT id, code, name, level, type, postal_code, created_at, updated_at, deleted_at
		FROM regions
		WHERE code = $1 AND deleted_at IS NULL
		LIMIT 1;
	`
	row := r.db.QueryRowContext(ctx, query, code)

	var reg domain.Region
	var regType sql.NullString
	var postalCode sql.NullString
	var deletedAt pq.NullTime

	err := row.Scan(
		&reg.ID,
		&reg.Code,
		&reg.Name,
		&reg.Level,
		&regType,
		&postalCode,
		&reg.CreatedAt,
		&reg.UpdatedAt,
		&deletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrRegionNotFound
		}
		return nil, fmt.Errorf("failed to get region by code: %w", err)
	}

	if regType.Valid {
		reg.Type = &regType.String
	}
	if postalCode.Valid {
		reg.PostalCode = &postalCode.String
	}
	if deletedAt.Valid {
		reg.DeletedAt = &deletedAt.Time
	}

	return &reg, nil
}

func (r *RegionRepository) GetListByLevel(ctx context.Context, level int16, filter domain.FilterParams) ([]domain.Region, int64, error) {
	page := filter.Page
	if page < 1 {
		page = 1
	}
	limit := filter.Limit
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	var whereClauses []string
	var args []interface{}
	argIdx := 1

	whereClauses = append(whereClauses, fmt.Sprintf("level = $%d", argIdx))
	args = append(args, level)
	argIdx++

	whereClauses = append(whereClauses, "deleted_at IS NULL")

	if strings.TrimSpace(filter.Search) != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("name ILIKE $%d", argIdx))
		args = append(args, "%"+strings.TrimSpace(filter.Search)+"%")
		argIdx++
	}

	whereSQL := strings.Join(whereClauses, " AND ")

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM regions WHERE %s;", whereSQL)
	var total int64
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count regions by level: %w", err)
	}

	// Strict Whitelist Mapping untuk OrderBy dan OrderDir
	safeOrderBy := "code"
	if filter.OrderBy == "name" {
		safeOrderBy = "name"
	}
	safeOrderDir := "ASC"
	if strings.ToUpper(filter.OrderDir) == "DESC" {
		safeOrderDir = "DESC"
	}

	selectQuery := fmt.Sprintf(`
		SELECT id, code, name, level, type, postal_code, created_at, updated_at
		FROM regions
		WHERE %s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d;
	`, whereSQL, safeOrderBy, safeOrderDir, argIdx, argIdx+1)

	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, selectQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to select regions by level: %w", err)
	}
	defer rows.Close()

	var regions []domain.Region
	for rows.Next() {
		var reg domain.Region
		var regType sql.NullString
		var postalCode sql.NullString
		if err := rows.Scan(
			&reg.ID,
			&reg.Code,
			&reg.Name,
			&reg.Level,
			&regType,
			&postalCode,
			&reg.CreatedAt,
			&reg.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan region row: %w", err)
		}
		if regType.Valid {
			reg.Type = &regType.String
		}
		if postalCode.Valid {
			reg.PostalCode = &postalCode.String
		}
		regions = append(regions, reg)
	}

	return regions, total, nil
}

func (r *RegionRepository) GetChildrenByPrefix(ctx context.Context, parentCode string, targetLevel int16, filter domain.FilterParams) ([]domain.Region, int64, error) {
	page := filter.Page
	if page < 1 {
		page = 1
	}
	limit := filter.Limit
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	var whereClauses []string
	var args []interface{}
	argIdx := 1

	// Prefix Pattern: parentCode + ".%"
	prefixPattern := strings.TrimSpace(parentCode) + ".%"
	whereClauses = append(whereClauses, fmt.Sprintf("code LIKE $%d", argIdx))
	args = append(args, prefixPattern)
	argIdx++

	whereClauses = append(whereClauses, fmt.Sprintf("level = $%d", argIdx))
	args = append(args, targetLevel)
	argIdx++

	whereClauses = append(whereClauses, "deleted_at IS NULL")

	if strings.TrimSpace(filter.Search) != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("name ILIKE $%d", argIdx))
		args = append(args, "%"+strings.TrimSpace(filter.Search)+"%")
		argIdx++
	}

	whereSQL := strings.Join(whereClauses, " AND ")

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM regions WHERE %s;", whereSQL)
	var total int64
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count children by prefix: %w", err)
	}

	// Strict Whitelist Mapping untuk OrderBy dan OrderDir
	safeOrderBy := "code"
	if filter.OrderBy == "name" {
		safeOrderBy = "name"
	}
	safeOrderDir := "ASC"
	if strings.ToUpper(filter.OrderDir) == "DESC" {
		safeOrderDir = "DESC"
	}

	selectQuery := fmt.Sprintf(`
		SELECT id, code, name, level, type, postal_code, created_at, updated_at
		FROM regions
		WHERE %s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d;
	`, whereSQL, safeOrderBy, safeOrderDir, argIdx, argIdx+1)

	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, selectQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to select children by prefix: %w", err)
	}
	defer rows.Close()

	var regions []domain.Region
	for rows.Next() {
		var reg domain.Region
		var regType sql.NullString
		var postalCode sql.NullString
		if err := rows.Scan(
			&reg.ID,
			&reg.Code,
			&reg.Name,
			&reg.Level,
			&regType,
			&postalCode,
			&reg.CreatedAt,
			&reg.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan child region row: %w", err)
		}
		if regType.Valid {
			reg.Type = &regType.String
		}
		if postalCode.Valid {
			reg.PostalCode = &postalCode.String
		}
		regions = append(regions, reg)
	}

	return regions, total, nil
}

func (r *RegionRepository) GetByPostalCode(ctx context.Context, postalCode string) (*domain.PostalHierarchy, error) {
	query := `
		SELECT id, code, name, level, type, created_at, updated_at
		FROM regions
		WHERE level = $1 AND name = $2 AND deleted_at IS NULL
		LIMIT 1;
	`
	var postalReg domain.Region
	var regType sql.NullString
	err := r.db.QueryRowContext(ctx, query, domain.LevelPostalCode, strings.TrimSpace(postalCode)).Scan(
		&postalReg.ID,
		&postalReg.Code,
		&postalReg.Name,
		&postalReg.Level,
		&regType,
		&postalReg.CreatedAt,
		&postalReg.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrRegionNotFound
		}
		return nil, fmt.Errorf("failed to find postal code: %w", err)
	}

	// Code format: province.regency.district.village.postal
	parts := strings.Split(postalReg.Code, ".")
	if len(parts) < 4 {
		return &domain.PostalHierarchy{PostalCode: postalCode}, nil
	}

	provCode := parts[0]
	regCode := strings.Join(parts[0:2], ".")
	distCode := strings.Join(parts[0:3], ".")
	vilCode := strings.Join(parts[0:4], ".")

	hierarchy := &domain.PostalHierarchy{
		PostalCode: postalCode,
	}

	if vil, err := r.GetByCode(ctx, vilCode); err == nil {
		hierarchy.Village = vil
	}
	if dist, err := r.GetByCode(ctx, distCode); err == nil {
		hierarchy.District = dist
	}
	if reg, err := r.GetByCode(ctx, regCode); err == nil {
		hierarchy.Regency = reg
	}
	if prov, err := r.GetByCode(ctx, provCode); err == nil {
		hierarchy.Province = prov
	}

	return hierarchy, nil
}

func (r *RegionRepository) GetPostalCodeByVillageCode(ctx context.Context, villageCode string) (*domain.VillagePostalCodeResponse, error) {
	query := `
		SELECT code, name, postal_code
		FROM regions
		WHERE code = $1 AND level = 4 AND deleted_at IS NULL
		LIMIT 1;
	`
	var resp domain.VillagePostalCodeResponse
	var postalCode sql.NullString

	err := r.db.QueryRowContext(ctx, query, strings.TrimSpace(villageCode)).Scan(
		&resp.VillageCode,
		&resp.VillageName,
		&postalCode,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrRegionNotFound
		}
		return nil, fmt.Errorf("failed to get postal code by village code: %w", err)
	}

	if postalCode.Valid {
		resp.PostalCode = &postalCode.String
	}

	return &resp, nil
}

func (r *RegionRepository) UpdatePostalCodeByVillageCode(ctx context.Context, villageCode, postalCode string) error {
	query := `
		UPDATE regions
		SET postal_code = $1, updated_at = NOW()
		WHERE code = $2 AND level = 4 AND deleted_at IS NULL;
	`
	cleanPostal := strings.TrimSpace(postalCode)
	res, err := r.db.ExecContext(ctx, query, cleanPostal, strings.TrimSpace(villageCode))
	if err != nil {
		return fmt.Errorf("failed to update postal code for village: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return domain.ErrRegionNotFound
	}

	return nil
}

func (r *RegionRepository) BatchUpdatePostalCodes(ctx context.Context, codes []string, postalCodes []string) (int64, error) {
	if len(codes) == 0 || len(postalCodes) == 0 || len(codes) != len(postalCodes) {
		return 0, nil
	}

	query := `
		UPDATE regions AS r
		SET postal_code = v.postal_code,
		    updated_at = NOW()
		FROM (
			SELECT unnest($1::text[]) AS code,
			       unnest($2::text[]) AS postal_code
		) AS v
		WHERE r.code = v.code
		  AND r.level = 4
		  AND r.deleted_at IS NULL;
	`

	res, err := r.db.ExecContext(ctx, query, pq.Array(codes), pq.Array(postalCodes))
	if err != nil {
		return 0, fmt.Errorf("failed to execute batch update postal codes: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get affected rows from batch update: %w", err)
	}

	return rows, nil
}

func (r *RegionRepository) Create(ctx context.Context, region *domain.Region) error {
	query := `
		INSERT INTO regions (code, name, level, type, postal_code, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at;
	`
	now := time.Now()
	err := r.db.QueryRowContext(
		ctx,
		query,
		region.Code,
		region.Name,
		region.Level,
		region.Type,
		region.PostalCode,
		now,
		now,
	).Scan(&region.ID, &region.CreatedAt, &region.UpdatedAt)

	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return domain.ErrRegionCodeAlreadyExists
		}
		return fmt.Errorf("failed to insert region: %w", err)
	}

	return nil
}

func (r *RegionRepository) Update(ctx context.Context, region *domain.Region) error {
	query := `
		UPDATE regions
		SET name = $1, type = $2, updated_at = $3
		WHERE code = $4 AND deleted_at IS NULL
		RETURNING id, level, created_at, updated_at;
	`
	now := time.Now()
	err := r.db.QueryRowContext(
		ctx,
		query,
		region.Name,
		region.Type,
		now,
		region.Code,
	).Scan(&region.ID, &region.Level, &region.CreatedAt, &region.UpdatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrRegionNotFound
		}
		return fmt.Errorf("failed to update region: %w", err)
	}

	return nil
}

func (r *RegionRepository) Delete(ctx context.Context, code string) error {
	query := `
		UPDATE regions
		SET deleted_at = $1
		WHERE code = $2 AND deleted_at IS NULL;
	`
	res, err := r.db.ExecContext(ctx, query, time.Now(), code)
	if err != nil {
		return fmt.Errorf("failed to soft delete region: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return domain.ErrRegionNotFound
	}

	return nil
}

func (r *RegionRepository) HasActiveChildren(ctx context.Context, parentCode string) (bool, error) {
	prefixPattern := strings.TrimSpace(parentCode) + ".%"
	query := `
		SELECT EXISTS(
			SELECT 1 FROM regions
			WHERE code LIKE $1 AND deleted_at IS NULL
		);
	`
	var exists bool
	err := r.db.QueryRowContext(ctx, query, prefixPattern).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check active children: %w", err)
	}

	return exists, nil
}
