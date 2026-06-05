package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type QuoteCache struct {
	client *redis.Client
	ttl    time.Duration
}

func NewQuoteCache(client *redis.Client, ttl time.Duration) *QuoteCache {
	return &QuoteCache{client: client, ttl: ttl}
}

func (c *QuoteCache) Get(ctx context.Context, key string, dest interface{}) (bool, error) {
	if c == nil || c.client == nil {
		return false, nil
	}
	data, err := c.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, json.Unmarshal(data, dest)
}

func (c *QuoteCache) Set(ctx context.Context, key string, value interface{}) error {
	return c.SetWithTTL(ctx, key, value, c.ttl)
}

func (c *QuoteCache) SetWithTTL(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if c == nil || c.client == nil {
		return nil
	}
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if ttl <= 0 {
		ttl = c.ttl
	}
	return c.client.Set(ctx, key, b, ttl).Err()
}

func QuoteKey(market, kind, code string) string {
	return fmt.Sprintf("warden:quote:%s:%s:%s", market, kind, code)
}

func QuotesKey(market, codes string) string {
	return fmt.Sprintf("warden:quotes:%s:%s", market, codes)
}
