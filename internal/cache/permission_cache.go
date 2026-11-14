package cache

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type cachedPermissions struct {
	perms     map[string]struct{}
	expiresAt time.Time
}

type PermissionCache struct {
	mu     sync.RWMutex
	items  map[string]cachedPermissions
	ttl    time.Duration
	stopCh chan struct{}
}

func NewPermissionCache(ttl, cleanupInterval time.Duration) *PermissionCache {
	c := &PermissionCache{
		items:  make(map[string]cachedPermissions),
		ttl:    ttl,
		stopCh: make(chan struct{}),
	}

	go c.startCleaner(cleanupInterval)
	return c
}

func (c *PermissionCache) cacheKey(tenantID, userID uuid.UUID) string {
	return fmt.Sprintf("%s:%s", tenantID.String(), userID.String())
}

func (c *PermissionCache) Get(tenantID, userID uuid.UUID) (map[string]struct{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := c.cacheKey(tenantID, userID)
	item, found := c.items[key]
	if !found {
		return nil, false
	}

	if time.Now().After(item.expiresAt) {
		return nil, false
	}

	return item.perms, true
}

func (c *PermissionCache) HasPermission(tenantID, userID uuid.UUID, permission string) (bool, bool) {
	perms, found := c.Get(tenantID, userID)
	if !found {
		return false, false
	}
	_, has := perms[permission]
	return has, true
}

func (c *PermissionCache) Set(tenantID, userID uuid.UUID, permissions []string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	set := make(map[string]struct{}, len(permissions))
	for _, p := range permissions {
		set[p] = struct{}{}
	}

	key := c.cacheKey(tenantID, userID)
	c.items[key] = cachedPermissions{
		perms:     set,
		expiresAt: time.Now().Add(c.ttl),
	}
}

func (c *PermissionCache) InvalidateUser(tenantID, userID uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := c.cacheKey(tenantID, userID)
	delete(c.items, key)
}

func (c *PermissionCache) InvalidateTenant(tenantID uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()

	prefix := tenantID.String() + ":"
	for k := range c.items {
		if strings.HasPrefix(k, prefix) {
			delete(c.items, k)
		}
	}
}

func (c *PermissionCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]cachedPermissions)
}

func (c *PermissionCache) Stop() {
	close(c.stopCh)
}

func (c *PermissionCache) startCleaner(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.mu.Lock()
			now := time.Now()
			for k, v := range c.items {
				if now.After(v.expiresAt) {
					delete(c.items, k)
				}
			}
			c.mu.Unlock()
		}
	}
}
