package cache

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type NonceStore interface {
	SetNX(ctx context.Context, key string, ttl time.Duration) (bool, error)
}

type RedisNonceStore struct {
	client *redis.Client
}

func NewRedis(client *redis.Client) *RedisNonceStore {
	return &RedisNonceStore{client: client}
}

func (r *RedisNonceStore) SetNX(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return r.client.SetNX(ctx, key, "1", ttl).Result()
}

func NewRedisClient(host string, port int, password string, db int) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", host, port),
		Password: password,
		DB:       db,
	})
}

// MemoryNonceStore is used when Redis is unavailable (dev/tests).
type MemoryNonceStore struct {
	mu   sync.Mutex
	keys map[string]time.Time
}

func NewMemoryNonceStore() *MemoryNonceStore {
	return &MemoryNonceStore{keys: make(map[string]time.Time)}
}

func (m *MemoryNonceStore) SetNX(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for k, exp := range m.keys {
		if now.After(exp) {
			delete(m.keys, k)
		}
	}
	if _, ok := m.keys[key]; ok {
		return false, nil
	}
	m.keys[key] = now.Add(ttl)
	return true, nil
}
