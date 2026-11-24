package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"be_region_indonesia/config"
	_ "github.com/lib/pq"
)

type PostgresPool struct {
	DB *sql.DB
}

// NewPostgresConnection menginisialisasi connection pool ke database PostgreSQL lokal
func NewPostgresConnection(cfg *config.Config) (*PostgresPool, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.DBHost,
		cfg.DBPort,
		cfg.DBUser,
		cfg.DBPassword,
		cfg.DBName,
		cfg.DBSSLMode,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Konfigurasi Connection Pooling
	db.SetMaxOpenConns(cfg.DBMaxOpenConns)
	db.SetMaxIdleConns(cfg.DBMaxIdleConns)
	db.SetConnMaxLifetime(time.Duration(cfg.DBConnMaxLifetimeM) * time.Minute)
	db.SetConnMaxIdleTime(time.Duration(cfg.DBConnMaxIdleTimeM) * time.Minute)

	// Verifikasi konektivitas dengan timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &PostgresPool{DB: db}, nil
}

func (p *PostgresPool) Close() error {
	if p.DB != nil {
		return p.DB.Close()
	}
	return nil
}
