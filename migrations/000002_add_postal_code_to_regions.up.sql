-- 1. Tambah kolom postal_code (Nullable, Default NULL)
-- Kolom dibuat NULL agar eksekusi DDL berjalan instan tanpa full table rewrite pada 83k+ baris.
ALTER TABLE regions ADD COLUMN IF NOT EXISTS postal_code VARCHAR(10) DEFAULT NULL;

-- 2. Partial Index pada postal_code
-- Hanya mengindeks baris yang memiliki nilai postal_code dan belum di-soft delete,
-- menghemat memori RAM dan mempercepat pencarian terbalik (reverse lookup) berdasarkan kode pos.
CREATE INDEX IF NOT EXISTS idx_regions_postal_code 
ON regions (postal_code) 
WHERE postal_code IS NOT NULL AND deleted_at IS NULL;

-- 3. Berikan komentar dokumentasi pada kolom
COMMENT ON COLUMN regions.postal_code IS 'Kode pos 5 digit wilayah administratif (diutamakan pada tingkat kelurahan/desa level 4)';
