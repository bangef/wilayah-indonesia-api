package migrator

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func TestScanMigrations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "migrations_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create valid migration pairs
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "000001_create_users.up.sql"), []byte("CREATE TABLE users();"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "000001_create_users.down.sql"), []byte("DROP TABLE users;"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "000002_add_email.up.sql"), []byte("ALTER TABLE users ADD email TEXT;"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "000002_add_email.down.sql"), []byte("ALTER TABLE users DROP COLUMN email;"), 0644))

	// Non-migration file should be ignored
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "README.md"), []byte("Docs"), 0644))

	files, err := ScanMigrations(tempDir)
	require.NoError(t, err)
	assert.Len(t, files, 2)
	assert.Equal(t, "000001", files[0].Version)
	assert.Equal(t, "000001_create_users", files[0].Name)
	assert.Equal(t, "000002", files[1].Version)
	assert.Equal(t, "000002_add_email", files[1].Name)

	// Test missing down file fails validation
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "000003_orphan.up.sql"), []byte("SELECT 1;"), 0644))
	_, err = ScanMigrations(tempDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "kehilangan berkas .down.sql")
}

func setupTestDB(t *testing.T) *sql.DB {
	_ = godotenv.Load("../../../.env")
	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "127.0.0.1"
	}
	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
		dbPort = "5432"
	}
	dbUser := os.Getenv("DB_USER")
	if dbUser == "" {
		dbUser = "postgres"
	}
	dbPass := os.Getenv("DB_PASSWORD")
	if dbPass == "" {
		dbPass = "postgres"
	}
	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "region_db"
	}
	dbSSL := os.Getenv("DB_SSLMODE")
	if dbSSL == "" {
		dbSSL = "disable"
	}

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		dbHost, dbPort, dbUser, dbPass, dbName, dbSSL)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Skipf("PostgreSQL tidak dapat dihubungkan: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Skipf("PostgreSQL ping gagal: %v", err)
	}
	return db
}

func TestMigratorAdvisoryLockAndLifecycle(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	tempDir, err := os.MkdirTemp("", "migrator_integration_*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	upSQL := `CREATE TABLE IF NOT EXISTS test_migration_dummy (id SERIAL PRIMARY KEY, note VARCHAR(50));`
	downSQL := `DROP TABLE IF EXISTS test_migration_dummy;`

	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "999901_test_dummy.up.sql"), []byte(upSQL), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "999901_test_dummy.down.sql"), []byte(downSQL), 0644))

	m := NewMigrator(db, tempDir)

	// Ensure advisory lock works
	err = m.Lock(ctx)
	require.NoError(t, err)
	err = m.Unlock(ctx)
	require.NoError(t, err)

	// Test EnsureMigrationTable
	err = m.EnsureMigrationTable(ctx)
	require.NoError(t, err)

	// Test Up
	count, err := m.Up(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Test Status
	statuses, err := m.Status(ctx)
	require.NoError(t, err)
	found := false
	for _, s := range statuses {
		if s.Version == "999901" {
			found = true
			assert.Equal(t, "APPLIED", s.Status)
			assert.NotNil(t, s.Batch)
		}
	}
	assert.True(t, found)

	// Test Rollback
	rolled, err := m.Rollback(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, 1, rolled)

	// Verify status after rollback
	statusesAfter, err := m.Status(ctx)
	require.NoError(t, err)
	for _, s := range statusesAfter {
		if s.Version == "999901" {
			assert.Equal(t, "PENDING", s.Status)
		}
	}
}

func TestMigratorTransactionalRollbackOnFailure(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	tempDir, err := os.MkdirTemp("", "migrator_fail_*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// DDL containing an intentional syntax error
	badSQL := `CREATE TABLE test_bad_table (id INT); INVALID_SQL_STATEMENT;`
	downSQL := `DROP TABLE IF EXISTS test_bad_table;`

	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "999902_test_bad.up.sql"), []byte(badSQL), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "999902_test_bad.down.sql"), []byte(downSQL), 0644))

	m := NewMigrator(db, tempDir)
	_, err = m.Up(ctx)
	assert.Error(t, err)

	// Verify table was not created and migration was not recorded
	var exists bool
	err = db.QueryRowContext(ctx, "SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'test_bad_table')").Scan(&exists)
	require.NoError(t, err)
	assert.False(t, exists, "Tabel test_bad_table seharusnya tidak terbentuk karena transaksi di-rollback")

	var recorded bool
	err = db.QueryRowContext(ctx, "SELECT EXISTS (SELECT FROM schema_migrations WHERE version = '999902')").Scan(&recorded)
	require.NoError(t, err)
	assert.False(t, recorded, "Versi 999902 tidak boleh tercatat di schema_migrations karena gagal")
}
