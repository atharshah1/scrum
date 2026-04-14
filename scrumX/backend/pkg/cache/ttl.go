package cache

import (
	"strings"
	"sync"
	"time"
)

type item struct {
	value     any
	expiresAt time.Time
}

type TTLCache struct {
	mu    sync.RWMutex
	items map[string]item
	ttl   time.Duration
}

func NewTTLCache(ttl time.Duration) *TTLCache {
	return &TTLCache{items: map[string]item{}, ttl: ttl}
}

func (c *TTLCache) Get(key string) (any, bool) {
	c.mu.RLock()
	it, ok := c.items[key]
	c.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if time.Now().After(it.expiresAt) {
		c.mu.Lock()
		if current, exists := c.items[key]; exists && !current.expiresAt.After(time.Now()) {
			delete(c.items, key)
		}
		c.mu.Unlock()
		return nil, false
	}
	return it.value, true
}

func (c *TTLCache) Set(key string, value any) {
	c.mu.Lock()
	c.items[key] = item{value: value, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}

func (c *TTLCache) Delete(key string) {
	c.mu.Lock()
	delete(c.items, key)
	c.mu.Unlock()
}

func (c *TTLCache) DeletePrefix(prefix string) {
	c.mu.Lock()
	for key := range c.items {
		if strings.HasPrefix(key, prefix) {
			delete(c.items, key)
		}
	}
	c.mu.Unlock()
}
