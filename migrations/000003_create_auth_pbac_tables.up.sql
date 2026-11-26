-- ============================================================================
-- 1. TABEL PENGGUNA (USERS)
-- ============================================================================
CREATE TABLE IF NOT EXISTS users (
    id VARCHAR(36) PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    email VARCHAR(100) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    region_scope VARCHAR(20) NOT NULL DEFAULT '*', -- '*' = Seluruh Indonesia, '32' = Jabar, '32.04' = Kab. Bandung
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL
);

CREATE INDEX IF NOT EXISTS idx_users_username ON users(username) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email) WHERE deleted_at IS NULL;

-- ============================================================================
-- 2. TABEL PERAN (ROLES)
-- ============================================================================
CREATE TABLE IF NOT EXISTS roles (
    id VARCHAR(36) PRIMARY KEY,
    code VARCHAR(50) UNIQUE NOT NULL, -- e.g. 'SUPERADMIN', 'REGIONAL_OPERATOR', 'REGIONAL_AUDITOR'
    name VARCHAR(100) NOT NULL,
    description TEXT,
    is_system BOOLEAN NOT NULL DEFAULT FALSE, -- Role sistem tidak boleh dihapus
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================================================
-- 3. TABEL HAK AKSES / IZIN (PERMISSIONS)
-- ============================================================================
CREATE TABLE IF NOT EXISTS permissions (
    id VARCHAR(36) PRIMARY KEY,
    code VARCHAR(100) UNIQUE NOT NULL, -- e.g. 'regions:read', 'regions:create', 'postal:update'
    module VARCHAR(50) NOT NULL,       -- e.g. 'REGIONS', 'POSTAL_CODE', 'AUTH_USERS'
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================================================
-- 4. PENGHUBUNG BANYAK-KE-BANYAK (USER_ROLES & ROLE_PERMISSIONS)
-- ============================================================================
CREATE TABLE IF NOT EXISTS user_roles (
    user_id VARCHAR(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id VARCHAR(36) NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, role_id)
);

CREATE TABLE IF NOT EXISTS role_permissions (
    role_id VARCHAR(36) NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id VARCHAR(36) NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (role_id, permission_id)
);

CREATE INDEX IF NOT EXISTS idx_user_roles_user_id ON user_roles(user_id);
CREATE INDEX IF NOT EXISTS idx_role_permissions_role_id ON role_permissions(role_id);

-- ============================================================================
-- 5. TABEL SESI REFRESH TOKEN (REFRESH_TOKENS) DENGAN REUSE DETECTION
-- ============================================================================
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(64) UNIQUE NOT NULL, -- SHA-256 dari plaintext random token
    family_id VARCHAR(36) NOT NULL,          -- UUID penanda rantai rotasi token
    is_revoked BOOLEAN NOT NULL DEFAULT FALSE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_hash ON refresh_tokens(token_hash);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_family ON refresh_tokens(family_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user ON refresh_tokens(user_id);

-- ============================================================================
-- 6. INITIAL SEEDER (BOOTSTRAP PERMISSIONS, ROLES & SUPERADMIN)
-- ============================================================================
-- A. Standard Permissions
INSERT INTO permissions (id, code, module, description) VALUES
    ('perm_reg_read',    'regions:read',    'REGIONS',     'Melihat daftar dan hierarki wilayah administratif'),
    ('perm_reg_create',  'regions:create',  'REGIONS',     'Membuat provinsi, kabupaten/kota, kecamatan, kelurahan'),
    ('perm_reg_update',  'regions:update',  'REGIONS',     'Memperbarui data dan nama wilayah'),
    ('perm_reg_delete',  'regions:delete',  'REGIONS',     'Menghapus wilayah administratif'),
    ('perm_post_update', 'postal:update',   'POSTAL_CODE', 'Memperbarui kode pos pada tingkat kelurahan/desa'),
    ('perm_post_import', 'postal:import',   'POSTAL_CODE', 'Melakukan impor massal kode pos via CSV'),
    ('perm_usr_manage',  'users:manage',    'AUTH_USERS',  'Mengelola pengguna sistem dan penugasan peran'),
    ('perm_rol_manage',  'roles:manage',    'AUTH_USERS',  'Membuat dan mengatur hak akses pada peran')
ON CONFLICT (code) DO NOTHING;

-- B. Default System Roles
INSERT INTO roles (id, code, name, description, is_system) VALUES
    ('role_superadmin', 'SUPERADMIN',        'Super Administrator', 'Akses penuh ke seluruh modul sistem dan wilayah', TRUE),
    ('role_reg_op',     'REGIONAL_OPERATOR', 'Operator Wilayah',    'Mengelola data wilayah dan kode pos dalam yurisdiksinya', FALSE),
    ('role_reg_audit',  'REGIONAL_AUDITOR',  'Auditor Wilayah',     'Akses baca laporan dan verifikasi data wilayah', FALSE)
ON CONFLICT (code) DO NOTHING;

-- C. Hubungkan Permissions ke Roles
INSERT INTO role_permissions (role_id, permission_id)
SELECT 'role_superadmin', id FROM permissions
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id) VALUES
    ('role_reg_op', 'perm_reg_read'),
    ('role_reg_op', 'perm_reg_create'),
    ('role_reg_op', 'perm_reg_update'),
    ('role_reg_op', 'perm_post_update'),
    ('role_reg_op', 'perm_post_import')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id) VALUES
    ('role_reg_audit', 'perm_reg_read')
ON CONFLICT DO NOTHING;

-- D. Default User Superadmin (Password: AdminRegional2026! di-hash Bcrypt cost 12)
INSERT INTO users (id, username, email, password_hash, region_scope, is_active) VALUES
    ('usr_root_superadmin', 'admin', 'admin@region.id', '$2b$12$uF3P65tPgKHB7C112DHueOQ8azEhP2iz0rkPhYuGVX9waRjxmyqo6', '*', TRUE)
ON CONFLICT (username) DO NOTHING;

INSERT INTO user_roles (user_id, role_id) VALUES
    ('usr_root_superadmin', 'role_superadmin')
ON CONFLICT DO NOTHING;
