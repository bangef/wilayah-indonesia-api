-- Rollback migrasi 000002
DROP INDEX IF EXISTS idx_regions_postal_code;
ALTER TABLE regions DROP COLUMN IF EXISTS postal_code;
