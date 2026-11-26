package migrator

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sort"
	"time"
)

const MigrationLockID int64 = 0x524547494F4E4D49 // "REGIONMI" in hex

type Migrator struct {
	db  *sql.DB
	dir string
}

func NewMigrator(db *sql.DB, migrationsDir string) *Migrator {
	return &Migrator{
		db:  db,
		dir: migrationsDir,
	}
}

// EnsureMigrationTable memastikan tabel schema_migrations tersedia
func (m *Migrator) EnsureMigrationTable(ctx context.Context) error {
	query := `
	CREATE TABLE IF NOT EXISTS schema_migrations (
		id SERIAL PRIMARY KEY,
		version VARCHAR(64) NOT NULL UNIQUE,
		migration_name VARCHAR(255) NOT NULL,
		batch INT NOT NULL,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_schema_migrations_batch ON schema_migrations (batch);
	CREATE INDEX IF NOT EXISTS idx_schema_migrations_version ON schema_migrations (version);
	`
	_, err := m.db.ExecContext(ctx, query)
	return err
}

// Lock mengakuisisi PostgreSQL session advisory lock
func (m *Migrator) Lock(ctx context.Context) error {
	_, err := m.db.ExecContext(ctx, "SELECT pg_advisory_lock($1)", MigrationLockID)
	return err
}

// Unlock melepaskan PostgreSQL session advisory lock
func (m *Migrator) Unlock(ctx context.Context) error {
	_, err := m.db.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", MigrationLockID)
	return err
}

// GetAppliedMigrations mengambil seluruh daftar migrasi yang sudah dieksekusi
func (m *Migrator) GetAppliedMigrations(ctx context.Context) (map[string]AppliedMigration, int, error) {
	rows, err := m.db.QueryContext(ctx, `SELECT id, version, migration_name, batch, applied_at::text FROM schema_migrations ORDER BY version ASC`)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	applied := make(map[string]AppliedMigration)
	maxBatch := 0

	for rows.Next() {
		var am AppliedMigration
		if err := rows.Scan(&am.ID, &am.Version, &am.Name, &am.Batch, &am.AppliedAt); err != nil {
			return nil, 0, err
		}
		applied[am.Version] = am
		if am.Batch > maxBatch {
			maxBatch = am.Batch
		}
	}
	return applied, maxBatch, rows.Err()
}

// Up mengeksekusi seluruh migrasi pending dalam batch baru
func (m *Migrator) Up(ctx context.Context) (int, error) {
	if err := m.Lock(ctx); err != nil {
		return 0, fmt.Errorf("gagal mengakuisisi advisory lock: %w", err)
	}
	defer m.Unlock(ctx)

	if err := m.EnsureMigrationTable(ctx); err != nil {
		return 0, fmt.Errorf("gagal menginisialisasi tabel migrasi: %w", err)
	}

	files, err := ScanMigrations(m.dir)
	if err != nil {
		return 0, err
	}

	applied, maxBatch, err := m.GetAppliedMigrations(ctx)
	if err != nil {
		return 0, err
	}

	nextBatch := maxBatch + 1
	migratedCount := 0

	for _, file := range files {
		if _, exists := applied[file.Version]; exists {
			continue // Sudah dieksekusi sebelumnya
		}

		fmt.Printf("[MIGRATE:UP] Mengeksekusi migrasi: %s (Batch: %d)...\n", file.Name, nextBatch)
		start := time.Now()

		content, err := os.ReadFile(file.UpPath)
		if err != nil {
			return migratedCount, fmt.Errorf("gagal membaca berkas %s: %w", file.UpPath, err)
		}

		tx, err := m.db.BeginTx(ctx, nil)
		if err != nil {
			return migratedCount, err
		}

		if _, err := tx.ExecContext(ctx, string(content)); err != nil {
			tx.Rollback()
			return migratedCount, fmt.Errorf("gagal mengeksekusi DDL berkas %s: %w", file.Name, err)
		}

		query := `INSERT INTO schema_migrations (version, migration_name, batch, applied_at) VALUES ($1, $2, $3, NOW())`
		if _, err := tx.ExecContext(ctx, query, file.Version, file.Name, nextBatch); err != nil {
			tx.Rollback()
			return migratedCount, fmt.Errorf("gagal mencatat riwayat migrasi %s: %w", file.Name, err)
		}

		if err := tx.Commit(); err != nil {
			return migratedCount, err
		}

		migratedCount++
		fmt.Printf("[MIGRATE:UP] Selesai: %s (%v)\n", file.Name, time.Since(start).Round(time.Millisecond))
	}

	if migratedCount == 0 {
		fmt.Println("[INFO] Tidak ada migrasi baru untuk dijalankan (Database schema up-to-date).")
	}
	return migratedCount, nil
}

// Rollback membatalkan migrasi dari N batch terakhir
func (m *Migrator) Rollback(ctx context.Context, steps int) (int, error) {
	if steps <= 0 {
		steps = 1
	}

	if err := m.Lock(ctx); err != nil {
		return 0, fmt.Errorf("gagal mengakuisisi advisory lock: %w", err)
	}
	defer m.Unlock(ctx)

	if err := m.EnsureMigrationTable(ctx); err != nil {
		return 0, err
	}

	files, err := ScanMigrations(m.dir)
	if err != nil {
		return 0, err
	}
	fileMap := make(map[string]MigrationFile)
	for _, f := range files {
		fileMap[f.Version] = f
	}

	// Dapatkan daftar batch unik secara descending
	rows, err := m.db.QueryContext(ctx, `SELECT DISTINCT batch FROM schema_migrations ORDER BY batch DESC`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var batches []int
	for rows.Next() {
		var b int
		if err := rows.Scan(&b); err != nil {
			return 0, err
		}
		batches = append(batches, b)
	}

	if len(batches) == 0 {
		fmt.Println("[INFO] Tidak ada migrasi yang tercatat untuk di-rollback.")
		return 0, nil
	}

	// Batasi sejumlah N batch
	targetBatches := batches
	if steps < len(batches) {
		targetBatches = batches[:steps]
	}

	// Ambil semua migrasi pada target batches, urutkan version DESC
	query := `SELECT version, migration_name, batch FROM schema_migrations WHERE batch >= $1 ORDER BY version DESC`
	minTargetBatch := targetBatches[len(targetBatches)-1]

	mRows, err := m.db.QueryContext(ctx, query, minTargetBatch)
	if err != nil {
		return 0, err
	}
	defer mRows.Close()

	type toRollback struct {
		Version string
		Name    string
		Batch   int
	}
	var rollbackList []toRollback
	for mRows.Next() {
		var item toRollback
		if err := mRows.Scan(&item.Version, &item.Name, &item.Batch); err != nil {
			return 0, err
		}
		rollbackList = append(rollbackList, item)
	}

	rolledBackCount := 0
	for _, item := range rollbackList {
		file, exists := fileMap[item.Version]
		if !exists {
			return rolledBackCount, fmt.Errorf("berkas .down.sql untuk migrasi versi %s tidak ditemukan di disk", item.Version)
		}

		fmt.Printf("[MIGRATE:ROLLBACK] Membatalkan migrasi: %s (Batch: %d)...\n", file.Name, item.Batch)
		start := time.Now()

		content, err := os.ReadFile(file.DownPath)
		if err != nil {
			return rolledBackCount, fmt.Errorf("gagal membaca berkas %s: %w", file.DownPath, err)
		}

		tx, err := m.db.BeginTx(ctx, nil)
		if err != nil {
			return rolledBackCount, err
		}

		if _, err := tx.ExecContext(ctx, string(content)); err != nil {
			tx.Rollback()
			return rolledBackCount, fmt.Errorf("gagal mengeksekusi DDL rollback %s: %w", file.Name, err)
		}

		if _, err := tx.ExecContext(ctx, `DELETE FROM schema_migrations WHERE version = $1`, file.Version); err != nil {
			tx.Rollback()
			return rolledBackCount, fmt.Errorf("gagal menghapus catatan migrasi %s: %w", file.Name, err)
		}

		if err := tx.Commit(); err != nil {
			return rolledBackCount, err
		}

		rolledBackCount++
		fmt.Printf("[MIGRATE:ROLLBACK] Selesai rollback: %s (%v)\n", file.Name, time.Since(start).Round(time.Millisecond))
	}

	return rolledBackCount, nil
}

// Reset membatalkan seluruh migrasi yang ada di database
func (m *Migrator) Reset(ctx context.Context) (int, error) {
	// 999999 steps artinya membatalkan seluruh batch
	return m.Rollback(ctx, 999999)
}

// Refresh me-reset seluruh migrasi lalu menjalankannya kembali dari awal
func (m *Migrator) Refresh(ctx context.Context) (int, error) {
	fmt.Println("[MIGRATE:REFRESH] Menginisiasi reset skema database...")
	if _, err := m.Reset(ctx); err != nil {
		return 0, fmt.Errorf("gagal melakukan reset skema database: %w", err)
	}

	fmt.Println("[MIGRATE:REFRESH] Menjalankan kembali seluruh berkas migrasi dari batch 1...")
	return m.Up(ctx)
}

// ToVersion mengarahkan migrasi ke target versi spesifik (maju atau mundur)
func (m *Migrator) ToVersion(ctx context.Context, targetVersion string) error {
	if err := m.Lock(ctx); err != nil {
		return err
	}
	defer m.Unlock(ctx)

	if err := m.EnsureMigrationTable(ctx); err != nil {
		return err
	}

	files, err := ScanMigrations(m.dir)
	if err != nil {
		return err
	}

	applied, maxBatch, err := m.GetAppliedMigrations(ctx)
	if err != nil {
		return err
	}

	// Validasi target version ada di disk atau 000000
	targetFound := false
	for _, f := range files {
		if f.Version == targetVersion {
			targetFound = true
			break
		}
	}
	if !targetFound && targetVersion != "000000" {
		return fmt.Errorf("target versi %s tidak valid atau berkas migrasinya tidak ditemukan", targetVersion)
	}

	// Tentukan apakah perlu migrasi UP atau ROLLBACK
	var pendingUp []MigrationFile
	for _, f := range files {
		if f.Version <= targetVersion {
			if _, exists := applied[f.Version]; !exists {
				pendingUp = append(pendingUp, f)
			}
		}
	}

	var pendingDown []MigrationFile
	for _, f := range files {
		if f.Version > targetVersion {
			if _, exists := applied[f.Version]; exists {
				pendingDown = append(pendingDown, f)
			}
		}
	}

	if len(pendingUp) == 0 && len(pendingDown) == 0 {
		fmt.Printf("[INFO] Database sudah berada pada target versi %s.\n", targetVersion)
		return nil
	}

	// Eksekusi Rollback jika ada versi di atas target
	if len(pendingDown) > 0 {
		sort.Slice(pendingDown, func(i, j int) bool {
			return pendingDown[i].Version > pendingDown[j].Version
		})
		for _, file := range pendingDown {
			fmt.Printf("[MIGRATE:VERSION-DOWN] Me-rollback migrasi: %s...\n", file.Name)
			content, err := os.ReadFile(file.DownPath)
			if err != nil {
				return err
			}
			tx, err := m.db.BeginTx(ctx, nil)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, string(content)); err != nil {
				tx.Rollback()
				return err
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM schema_migrations WHERE version = $1`, file.Version); err != nil {
				tx.Rollback()
				return err
			}
			if err := tx.Commit(); err != nil {
				return err
			}
		}
	}

	// Eksekusi Up jika ada versi di bawah/sama dengan target
	if len(pendingUp) > 0 {
		nextBatch := maxBatch + 1
		for _, file := range pendingUp {
			fmt.Printf("[MIGRATE:VERSION-UP] Mengeksekusi migrasi: %s (Batch: %d)...\n", file.Name, nextBatch)
			content, err := os.ReadFile(file.UpPath)
			if err != nil {
				return err
			}
			tx, err := m.db.BeginTx(ctx, nil)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, string(content)); err != nil {
				tx.Rollback()
				return err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version, migration_name, batch, applied_at) VALUES ($1, $2, $3, NOW())`, file.Version, file.Name, nextBatch); err != nil {
				tx.Rollback()
				return err
			}
			if err := tx.Commit(); err != nil {
				return err
			}
		}
	}

	fmt.Printf("[MIGRATE:VERSION] Sukses menyinkronkan skema database ke versi %s.\n", targetVersion)
	return nil
}

// Status menampilkan ikhtisar status migrasi terformat
func (m *Migrator) Status(ctx context.Context) ([]MigrationStatus, error) {
	if err := m.EnsureMigrationTable(ctx); err != nil {
		return nil, err
	}

	files, err := ScanMigrations(m.dir)
	if err != nil {
		return nil, err
	}

	applied, _, err := m.GetAppliedMigrations(ctx)
	if err != nil {
		return nil, err
	}

	var statuses []MigrationStatus
	for _, f := range files {
		st := MigrationStatus{
			Version: f.Version,
			Name:    f.Name,
		}
		if am, exists := applied[f.Version]; exists {
			b := am.Batch
			appAt := am.AppliedAt
			st.Batch = &b
			st.AppliedAt = &appAt
			st.Status = "APPLIED"
		} else {
			st.Status = "PENDING"
		}
		statuses = append(statuses, st)
	}
	return statuses, nil
}
