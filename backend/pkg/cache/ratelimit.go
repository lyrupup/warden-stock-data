package cache

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type RateLimiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error)
}

type RedisRateLimiter struct {
	client *redis.Client
}

func NewRedisRateLimiter(client *redis.Client) *RedisRateLimiter {
	return &RedisRateLimiter{client: client}
}

func (r *RedisRateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	pipe := r.client.TxPipeline()
	cnt := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, window)
	if _, err := pipe.Exec(ctx); err != nil {
		return false, err
	}
	n, err := cnt.Result()
	if err != nil {
		return false, err
	}
	return n <= int64(limit), nil
}

type MemoryRateLimiter struct {
	mu    sync.Mutex
	count map[string]int
	reset map[string]time.Time
}

func NewMemoryRateLimiter() *MemoryRateLimiter {
	return &MemoryRateLimiter{count: make(map[string]int), reset: make(map[string]time.Time)}
}

func (m *MemoryRateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	if exp, ok := m.reset[key]; !ok || now.After(exp) {
		m.count[key] = 0
		m.reset[key] = now.Add(window)
	}
	m.count[key]++
	return m.count[key] <= limit, nil
}

type QuotaStore interface {
	IncrDaily(ctx context.Context, key string) (int64, error)
}

type RedisQuotaStore struct {
	client *redis.Client
}

func NewRedisQuota(client *redis.Client) *RedisQuotaStore {
	return &RedisQuotaStore{client: client}
}

func (r *RedisQuotaStore) IncrDaily(ctx context.Context, key string) (int64, error) {
	n, err := r.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	if n == 1 {
		now := time.Now()
		end := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())
		_ = r.client.ExpireAt(ctx, key, end).Err()
	}
	return n, nil
}

type MemoryQuotaStore struct {
	mu    sync.Mutex
	count map[string]int64
	day   string
}

func NewMemoryQuotaStore() *MemoryQuotaStore {
	return &MemoryQuotaStore{count: make(map[string]int64)}
}

func (m *MemoryQuotaStore) IncrDaily(ctx context.Context, key string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	today := time.Now().Format("2006-01-02")
	if m.day != today {
		m.count = make(map[string]int64)
		m.day = today
	}
	m.count[key]++
	return m.count[key], nil
}

func RateLimitKey(credentialID uint) string {
	return fmt.Sprintf("warden:ratelimit:%d", credentialID)
}

func QuotaKey(credentialID uint) string {
	today := time.Now().Format("2006-01-02")
	return fmt.Sprintf("warden:quota:%d:%s", credentialID, today)
}
