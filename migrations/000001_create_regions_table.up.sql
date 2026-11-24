-- Extension for Trigram search (fuzzy search & autocomplete nama wilayah)
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- Single Table: regions
CREATE TABLE IF NOT EXISTS regions (
    id BIGSERIAL PRIMARY KEY,
    code VARCHAR(100) NOT NULL,
    name VARCHAR(255) NOT NULL,
    level SMALLINT NOT NULL CHECK (level BETWEEN 1 AND 5),
    type VARCHAR(50) DEFAULT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- 1. Unique constraint pada code (hanya record yang aktif / belum di-soft delete)
CREATE UNIQUE INDEX IF NOT EXISTS idx_regions_code_unique ON regions (code) WHERE deleted_at IS NULL;

-- 2. Index untuk pola query prefix LIKE 'xx.%' menggunakan operator class 'varchar_pattern_ops'
CREATE INDEX IF NOT EXISTS idx_regions_code_pattern ON regions (code varchar_pattern_ops) WHERE deleted_at IS NULL;

-- 3. Composite Index (level + code) untuk filtering cepat berdasarkan level dan prefix
CREATE INDEX IF NOT EXISTS idx_regions_level_code_pattern ON regions (level, code varchar_pattern_ops) WHERE deleted_at IS NULL;

-- 4. Trigram GIN index untuk pencarian teks nama wilayah (autocomplete)
CREATE INDEX IF NOT EXISTS idx_regions_name_trgm ON regions USING gin (name gin_trgm_ops) WHERE deleted_at IS NULL;

-- 5. Index pada field 'name' saat level = 5 (pencarian langsung berdasarkan 5-digit kode pos)
CREATE INDEX IF NOT EXISTS idx_regions_postal_name ON regions (name) WHERE level = 5 AND deleted_at IS NULL;
