package cache_test

import (
	"sync"
	"testing"
	"time"

	"github.com/deb151292/neurealm-go-Assignment-/internal/cache"
	"github.com/google/uuid"
)

func TestPermissionCache_SetAndGet(t *testing.T) {
	c := cache.NewPermissionCache(1*time.Minute, 5*time.Minute)
	defer c.Stop()

	tenantID := uuid.New()
	userID := uuid.New()
	permissions := []string{"user:create", "user:read", "role:assign"}

	// Before set
	perms, found := c.Get(tenantID, userID)
	if found || perms != nil {
		t.Fatalf("expected cache miss before set, got %v", perms)
	}

	// Set
	c.Set(tenantID, userID, permissions)

	// After set
	perms, found = c.Get(tenantID, userID)
	if !found {
		t.Fatalf("expected cache hit after set")
	}
	if len(perms) != 3 {
		t.Fatalf("expected 3 permissions, got %d", len(perms))
	}

	// HasPermission check
	has, found := c.HasPermission(tenantID, userID, "user:create")
	if !found || !has {
		t.Errorf("expected user to have user:create permission")
	}

	has, found = c.HasPermission(tenantID, userID, "tenant:delete")
	if !found || has {
		t.Errorf("expected user NOT to have tenant:delete permission")
	}
}

func TestPermissionCache_InvalidateUser(t *testing.T) {
	c := cache.NewPermissionCache(1*time.Minute, 5*time.Minute)
	defer c.Stop()

	tenantID := uuid.New()
	user1 := uuid.New()
	user2 := uuid.New()

	c.Set(tenantID, user1, []string{"user:read"})
	c.Set(tenantID, user2, []string{"user:read"})

	// Invalidate user1
	c.InvalidateUser(tenantID, user1)

	_, found1 := c.Get(tenantID, user1)
	if found1 {
		t.Errorf("user1 should be invalidated from cache")
	}

	_, found2 := c.Get(tenantID, user2)
	if !found2 {
		t.Errorf("user2 should still be in cache")
	}
}

func TestPermissionCache_InvalidateTenant(t *testing.T) {
	c := cache.NewPermissionCache(1*time.Minute, 5*time.Minute)
	defer c.Stop()

	tenant1 := uuid.New()
	tenant2 := uuid.New()
	user := uuid.New()

	c.Set(tenant1, user, []string{"user:read"})
	c.Set(tenant2, user, []string{"user:read"})

	c.InvalidateTenant(tenant1)

	_, found1 := c.Get(tenant1, user)
	if found1 {
		t.Errorf("tenant1 should be invalidated")
	}

	_, found2 := c.Get(tenant2, user)
	if !found2 {
		t.Errorf("tenant2 should still be in cache")
	}
}

func TestPermissionCache_Expiration(t *testing.T) {
	// 50ms TTL
	c := cache.NewPermissionCache(50*time.Millisecond, 100*time.Millisecond)
	defer c.Stop()

	tenantID := uuid.New()
	userID := uuid.New()

	c.Set(tenantID, userID, []string{"user:create"})

	_, found := c.Get(tenantID, userID)
	if !found {
		t.Fatalf("expected cache hit immediately after set")
	}

	time.Sleep(70 * time.Millisecond)

	_, found = c.Get(tenantID, userID)
	if found {
		t.Fatalf("expected cache item to have expired")
	}
}

func TestPermissionCache_Concurrency(t *testing.T) {
	c := cache.NewPermissionCache(1*time.Minute, 5*time.Minute)
	defer c.Stop()

	tenantID := uuid.New()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			uid := uuid.New()
			c.Set(tenantID, uid, []string{"user:read", "user:write"})
			_, _ = c.Get(tenantID, uid)
			c.InvalidateUser(tenantID, uid)
		}()
	}

	wg.Wait()
}
