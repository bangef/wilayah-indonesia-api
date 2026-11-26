package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"be_region_indonesia/internal/domain"
)

type authRepository struct {
	db *sql.DB
}

func NewAuthRepository(db *sql.DB) domain.AuthRepository {
	return &authRepository{db: db}
}

func (r *authRepository) GetUserByUsername(ctx context.Context, username string) (*domain.User, string, error) {
	query := `
		SELECT id, username, email, password_hash, region_scope, is_active, created_at, updated_at
		FROM users
		WHERE username = $1 AND deleted_at IS NULL
	`
	var u domain.User
	var passwordHash string
	err := r.db.QueryRowContext(ctx, query, username).Scan(
		&u.ID, &u.Username, &u.Email, &passwordHash, &u.RegionScope, &u.IsActive, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", domain.ErrUserNotFound
		}
		return nil, "", fmt.Errorf("failed to get user by username: %w", err)
	}

	roles, err := r.getUserRoles(ctx, u.ID)
	if err == nil {
		u.Roles = roles
	}

	return &u, passwordHash, nil
}

func (r *authRepository) GetUserByID(ctx context.Context, userID string) (*domain.User, error) {
	query := `
		SELECT id, username, email, region_scope, is_active, created_at, updated_at
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`
	var u domain.User
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&u.ID, &u.Username, &u.Email, &u.RegionScope, &u.IsActive, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user by id: %w", err)
	}

	roles, err := r.getUserRoles(ctx, u.ID)
	if err == nil {
		u.Roles = roles
	}

	return &u, nil
}

func (r *authRepository) getUserRoles(ctx context.Context, userID string) ([]domain.Role, error) {
	query := `
		SELECT r.id, r.code, r.name, COALESCE(r.description, ''), r.is_system
		FROM roles r
		INNER JOIN user_roles ur ON ur.role_id = r.id
		WHERE ur.user_id = $1
	`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []domain.Role
	for rows.Next() {
		var role domain.Role
		if err := rows.Scan(&role.ID, &role.Code, &role.Name, &role.Description, &role.IsSystem); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

func (r *authRepository) GetUserPermissions(ctx context.Context, userID string) ([]string, error) {
	query := `
		SELECT DISTINCT p.code
		FROM permissions p
		INNER JOIN role_permissions rp ON rp.permission_id = p.id
		INNER JOIN user_roles ur ON ur.role_id = rp.role_id
		WHERE ur.user_id = $1
	`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user permissions: %w", err)
	}
	defer rows.Close()

	var perms []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		perms = append(perms, code)
	}
	return perms, rows.Err()
}

func (r *authRepository) SaveRefreshToken(ctx context.Context, token *domain.RefreshToken) error {
	query := `
		INSERT INTO refresh_tokens (id, user_id, token_hash, family_id, is_revoked, expires_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.db.ExecContext(ctx, query,
		token.ID, token.UserID, token.TokenHash, token.FamilyID, token.IsRevoked, token.ExpiresAt, token.CreatedAt, token.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to save refresh token: %w", err)
	}
	return nil
}

func (r *authRepository) GetRefreshTokenByHash(ctx context.Context, tokenHash string) (*domain.RefreshToken, error) {
	query := `
		SELECT id, user_id, token_hash, family_id, is_revoked, expires_at, created_at, updated_at
		FROM refresh_tokens
		WHERE token_hash = $1
	`
	var t domain.RefreshToken
	err := r.db.QueryRowContext(ctx, query, tokenHash).Scan(
		&t.ID, &t.UserID, &t.TokenHash, &t.FamilyID, &t.IsRevoked, &t.ExpiresAt, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrInvalidRefreshToken
		}
		return nil, fmt.Errorf("failed to get refresh token: %w", err)
	}
	return &t, nil
}

func (r *authRepository) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	query := `UPDATE refresh_tokens SET is_revoked = TRUE, updated_at = NOW() WHERE token_hash = $1`
	_, err := r.db.ExecContext(ctx, query, tokenHash)
	if err != nil {
		return fmt.Errorf("failed to revoke refresh token: %w", err)
	}
	return nil
}

func (r *authRepository) RevokeTokenFamily(ctx context.Context, familyID string) error {
	query := `UPDATE refresh_tokens SET is_revoked = TRUE, updated_at = NOW() WHERE family_id = $1`
	_, err := r.db.ExecContext(ctx, query, familyID)
	if err != nil {
		return fmt.Errorf("failed to revoke token family: %w", err)
	}
	return nil
}

func (r *authRepository) RevokeAllUserRefreshTokens(ctx context.Context, userID string) error {
	query := `UPDATE refresh_tokens SET is_revoked = TRUE, updated_at = NOW() WHERE user_id = $1`
	_, err := r.db.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to revoke all user refresh tokens: %w", err)
	}
	return nil
}
