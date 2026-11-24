package database_test

import (
	"path/filepath"
	"testing"

	"be_region_indonesia/config"
	"be_region_indonesia/pkg/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostgresConnection(t *testing.T) {
	rootEnv := filepath.Join("..", "..", ".env")
	cfg, err := config.LoadConfig(rootEnv)
	require.NoError(t, err)

	pool, err := database.NewPostgresConnection(cfg)
	require.NoError(t, err)
	require.NotNil(t, pool)
	defer pool.Close()

	var result int
	err = pool.DB.QueryRow("SELECT 1;").Scan(&result)
	assert.NoError(t, err)
	assert.Equal(t, 1, result)
}
