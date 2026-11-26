package cache

import (
	"context"
	"fmt"
	"time"

	"be_region_indonesia/internal/domain"

	"github.com/redis/go-redis/v9"
)

type redisAuthCache struct {
	client *redis.Client
}

func NewRedisAuthCache(client *redis.Client) domain.AuthCacheRepository {
	return &redisAuthCache{client: client}
}

func (c *redisAuthCache) permKey(userID string) string {
	return fmt.Sprintf("auth:perms:%s", userID)
}

func (c *redisAuthCache) blacklistKey(jti string) string {
	return fmt.Sprintf("jwt:blacklist:%s", jti)
}

func (c *redisAuthCache) GetUserPermissions(ctx context.Context, userID string) ([]string, bool, error) {
	key := c.permKey(userID)
	exists, err := c.client.Exists(ctx, key).Result()
	if err != nil || exists == 0 {
		return nil, false, err
	}

	perms, err := c.client.SMembers(ctx, key).Result()
	if err != nil {
		return nil, false, err
	}
	return perms, true, nil
}

func (c *redisAuthCache) SetUserPermissions(ctx context.Context, userID string, permissions []string, ttl time.Duration) error {
	if len(permissions) == 0 {
		return nil
	}
	key := c.permKey(userID)

	pipe := c.client.Pipeline()
	pipe.Del(ctx, key)
	args := make([]interface{}, len(permissions))
	for i, p := range permissions {
		args[i] = p
	}
	pipe.SAdd(ctx, key, args...)
	if ttl > 0 {
		pipe.Expire(ctx, key, ttl)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (c *redisAuthCache) InvalidateUserPermissions(ctx context.Context, userID string) error {
	return c.client.Del(ctx, c.permKey(userID)).Err()
}

func (c *redisAuthCache) InvalidateAllPermissions(ctx context.Context) error {
	var cursor uint64
	for {
		keys, nextCursor, err := c.client.Scan(ctx, cursor, "auth:perms:*", 100).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := c.client.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return nil
}

func (c *redisAuthCache) BlacklistAccessToken(ctx context.Context, jti string, ttl time.Duration) error {
	if jti == "" || ttl <= 0 {
		return nil
	}
	return c.client.Set(ctx, c.blacklistKey(jti), "revoked", ttl).Err()
}

func (c *redisAuthCache) IsAccessTokenBlacklisted(ctx context.Context, jti string) (bool, error) {
	if jti == "" {
		return false, nil
	}
	res, err := c.client.Exists(ctx, c.blacklistKey(jti)).Result()
	if err != nil {
		return false, err
	}
	return res > 0, nil
}
