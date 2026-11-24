package database

import (
	"context"
	"fmt"
	"time"

	"be_region_indonesia/config"
	"github.com/redis/go-redis/v9"
)

type RedisPool struct {
	Client *redis.Client
}

// NewRedisConnection menginisialisasi client pool ke Redis lokal
func NewRedisConnection(cfg *config.Config) (*RedisPool, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", cfg.RedisHost, cfg.RedisPort),
		Password:     cfg.RedisPassword,
		DB:           cfg.RedisDB,
		PoolSize:     cfg.RedisPoolSize,
		MinIdleConns: 10,
		DialTimeout:  3 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		return nil, fmt.Errorf("failed to ping redis: %w", err)
	}

	return &RedisPool{Client: rdb}, nil
}

func (p *RedisPool) Close() error {
	if p.Client != nil {
		return p.Client.Close()
	}
	return nil
}
