package cache

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type item struct {
	value     any
	expiresAt time.Time
}

type TTLCache struct {
	mu     sync.RWMutex
	items  map[string]item
	ttl    time.Duration
	redis  *redis.Client
	remote bool
}

func NewTTLCache(ttl time.Duration) *TTLCache {
	return &TTLCache{items: map[string]item{}, ttl: ttl}
}

func NewRedisBackedTTLCache(ttl time.Duration, addr, password string, db int) *TTLCache {
	c := NewTTLCache(ttl)
	if strings.TrimSpace(addr) == "" {
		return c
	}
	c.redis = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	c.remote = true
	return c
}

func (c *TTLCache) Get(key string) (any, bool) {
	if c.remote && c.redis != nil {
		raw, err := c.redis.Get(context.Background(), key).Bytes()
		if err == redis.Nil || err != nil {
			return nil, false
		}
		var v any
		if err := json.Unmarshal(raw, &v); err != nil {
			_ = c.redis.Del(context.Background(), key).Err()
			return nil, false
		}
		return v, true
	}
	c.mu.RLock()
	it, ok := c.items[key]
	c.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if time.Now().After(it.expiresAt) {
		c.mu.Lock()
		if current, exists := c.items[key]; exists && current.expiresAt.Equal(it.expiresAt) {
			delete(c.items, key)
		}
		c.mu.Unlock()
		return nil, false
	}
	return it.value, true
}

func (c *TTLCache) Set(key string, value any) {
	if c.remote && c.redis != nil {
		raw, err := json.Marshal(value)
		if err != nil {
			return
		}
		_ = c.redis.Set(context.Background(), key, raw, c.ttl).Err()
		return
	}
	c.mu.Lock()
	c.items[key] = item{value: value, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}

func (c *TTLCache) Delete(key string) {
	if c.remote && c.redis != nil {
		_ = c.redis.Del(context.Background(), key).Err()
		return
	}
	c.mu.Lock()
	delete(c.items, key)
	c.mu.Unlock()
}

func (c *TTLCache) DeletePrefix(prefix string) {
	if c.remote && c.redis != nil {
		ctx := context.Background()
		iter := c.redis.Scan(ctx, 0, prefix+"*", 100).Iterator()
		keys := make([]string, 0, 100)
		for iter.Next(ctx) {
			keys = append(keys, iter.Val())
			if len(keys) >= 100 {
				_ = c.redis.Del(ctx, keys...).Err()
				keys = keys[:0]
			}
		}
		if len(keys) > 0 {
			_ = c.redis.Del(ctx, keys...).Err()
		}
		return
	}
	c.mu.Lock()
	for key := range c.items {
		if strings.HasPrefix(key, prefix) {
			delete(c.items, key)
		}
	}
	c.mu.Unlock()
}

func (c *TTLCache) Ping(ctx context.Context) error {
	if c.remote && c.redis != nil {
		return c.redis.Ping(ctx).Err()
	}
	return nil
}

func (c *TTLCache) Close() error {
	if c.redis == nil {
		return nil
	}
	return c.redis.Close()
}
