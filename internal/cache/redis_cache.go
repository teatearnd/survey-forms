package cache

import (
	"context"
	"fmt"

	"github.com/go-redis/redis/v8"
)

type RedisCache struct {
	client *redis.Client
	ctx    context.Context
}

func NewRedisCache(addr, password string, db int) *RedisCache {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	return &RedisCache{client: rdb, ctx: context.Background()}
}

func (r *RedisCache) Ping() error {
	_, err := r.client.Ping(context.Background()).Result()
	return err
}

func cartkey(userID string) string {
	return fmt.Sprintf("cart:%s", userID)
}

func (r *RedisCache) AddItem(userID string, item any) error {
	return r.client.LPush(r.ctx, cartkey(userID), item).Err()
}

func (r *RedisCache) GetItems(userID string, limit, offset int) ([]string, error) {
	start := int64(offset)
	end := int64(offset + limit - 1)

	vals, err := r.client.LRange(r.ctx, cartkey(userID), start, end).Result()
	if err != nil {
		return nil, fmt.Errorf("failed while getting items: %w", err)
	}
	return vals, err
}

// RemoveItemByIndex removes item at index using LSET and LREM
func (r *RedisCache) RemoveItemByIndex(userID string, index int) error {
	key := cartkey(userID)
	marker := "__TO_DELETE__"
	err := r.client.LSet(r.ctx, key, int64(index), marker).Err()
	if err != nil {
		return fmt.Errorf("failed to remove the item by idx: %w", err)
	}
	// remove first occurence of the marker
	return r.client.LRem(r.ctx, key, 1, marker).Err()
}

// Len returns number of items in the cart
func (r *RedisCache) Len(userID string) (int64, error) {
	return r.client.LLen(r.ctx, cartkey(userID)).Result()
}

// Fully wipes the cart with its items
func (r *RedisCache) ClearCart(userID string) error {
	if err := r.client.Del(r.ctx, cartkey(userID)).Err(); err != nil {
		return fmt.Errorf("failed to clear cart: %w", err)
	}
	return nil
}
