package service

import (
	"context"
	"sync"
	"time"

	"github.com/warden-stock/warden-stock-data/internal/model"
)

type cachedCredential struct {
	cred      *model.APICredential
	secretKey string
	expiresAt time.Time
}

type credentialCache struct {
	mu    sync.RWMutex
	items map[uint]cachedCredential
	bySID map[string]uint
	ttl   time.Duration
}

func newCredentialCache(ttl time.Duration) *credentialCache {
	return &credentialCache{
		items: make(map[uint]cachedCredential),
		bySID: make(map[string]uint),
		ttl:   ttl,
	}
}

func (c *credentialCache) set(id uint, cred *model.APICredential, secretKey string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[id] = cachedCredential{cred: cred, secretKey: secretKey, expiresAt: time.Now().Add(c.ttl)}
	c.bySID[cred.SecretID] = id
}

func (c *credentialCache) getByID(id uint) (cachedCredential, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	item, ok := c.items[id]
	if !ok || time.Now().After(item.expiresAt) {
		return cachedCredential{}, false
	}
	return item, true
}

func (c *credentialCache) getBySecretID(secretID string) (cachedCredential, bool) {
	c.mu.RLock()
	id, ok := c.bySID[secretID]
	c.mu.RUnlock()
	if !ok {
		return cachedCredential{}, false
	}
	return c.getByID(id)
}

func (c *credentialCache) invalidate(id uint) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if item, ok := c.items[id]; ok {
		delete(c.bySID, item.cred.SecretID)
	}
	delete(c.items, id)
}

func (s *CredentialService) enableCache() {
	if s.cache == nil {
		s.cache = newCredentialCache(60 * time.Second)
	}
}

// GetCached returns credential by id from memory cache or DB.
func (s *CredentialService) GetCached(ctx context.Context, id uint) (*model.APICredential, error) {
	s.enableCache()
	if item, ok := s.cache.getByID(id); ok {
		return item.cred, nil
	}
	return s.Get(ctx, id)
}

func (s *CredentialService) ResolveSecretKeyCached(ctx context.Context, secretID string) (string, *model.APICredential, error) {
	s.enableCache()
	if item, ok := s.cache.getBySecretID(secretID); ok {
		return item.secretKey, item.cred, nil
	}
	key, cred, err := s.ResolveSecretKey(ctx, secretID)
	if err != nil {
		return "", nil, err
	}
	s.cache.set(cred.ID, cred, key)
	return key, cred, nil
}
