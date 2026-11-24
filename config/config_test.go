package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"be_region_indonesia/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_Success(t *testing.T) {
	tempDir := t.TempDir()
	envFile := filepath.Join(tempDir, ".env")
	envContent := `
APP_NAME=test_service
APP_ENV=development
APP_PORT=9090
DB_HOST=127.0.0.1
DB_PORT=5432
DB_USER=test_user
DB_PASSWORD=test_pass
DB_NAME=test_db
REDIS_HOST=127.0.0.1
REDIS_PORT=6379
JWT_SECRET=this-is-a-32-characters-test-secret-key
`
	err := os.WriteFile(envFile, []byte(envContent), 0600)
	require.NoError(t, err)

	cfg, err := config.LoadConfig(envFile)
	require.NoError(t, err)
	assert.Equal(t, "test_service", cfg.AppName)
	assert.Equal(t, 9090, cfg.AppPort)
	assert.Equal(t, "test_user", cfg.DBUser)
	assert.Equal(t, "test_db", cfg.DBName)
	assert.Equal(t, 6379, cfg.RedisPort)
}

func TestLoadConfig_ValidationError_InvalidPort(t *testing.T) {
	tempDir := t.TempDir()
	envFile := filepath.Join(tempDir, ".env")
	envContent := `
APP_NAME=test_service
APP_PORT=999999
`
	err := os.WriteFile(envFile, []byte(envContent), 0600)
	require.NoError(t, err)

	_, err = config.LoadConfig(envFile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "APP_PORT")
}

func TestLoadConfig_ProductionValidation_StrictSecrets(t *testing.T) {
	tempDir := t.TempDir()
	envFile := filepath.Join(tempDir, ".env")
	envContent := `
APP_NAME=test_service
APP_ENV=production
APP_PORT=8080
DB_HOST=127.0.0.1
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=
DB_NAME=prod_db
REDIS_HOST=127.0.0.1
REDIS_PORT=6379
JWT_SECRET=short
`
	err := os.WriteFile(envFile, []byte(envContent), 0600)
	require.NoError(t, err)

	_, err = config.LoadConfig(envFile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "DB_PASSWORD (required in production)")
	assert.Contains(t, err.Error(), "JWT_SECRET (must be >= 32 characters in production)")
}
