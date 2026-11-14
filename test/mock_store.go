package test

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/neurealm/multi-tenant-system/internal/domain"
)

// MockStore provides an in-memory thread-safe implementation of all repository interfaces
type MockStore struct {
	mu           sync.RWMutex
	Tenants      map[uuid.UUID]*domain.Tenant
	Users        map[uuid.UUID]*domain.User
	TenantUsers  map[string]*domain.TenantUser // key: "tenantID:userID"
	Roles        map[uuid.UUID]*domain.Role
	RolePerms    map[uuid.UUID][]uuid.UUID // roleID -> []permissionID
	UserRoles    map[string][]uuid.UUID    // "tenantID:userID" -> []roleID
	Permissions  map[uuid.UUID]*domain.Permission
	RefreshTokens map[string]*domain.RefreshToken // key: tokenHash
	AuditLogs    []domain.AuditLog
}

func NewMockStore() *MockStore {
	store := &MockStore{
		Tenants:       make(map[uuid.UUID]*domain.Tenant),
		Users:         make(map[uuid.UUID]*domain.User),
		TenantUsers:   make(map[string]*domain.TenantUser),
		Roles:         make(map[uuid.UUID]*domain.Role),
		RolePerms:     make(map[uuid.UUID][]uuid.UUID),
		UserRoles:     make(map[string][]uuid.UUID),
		Permissions:   make(map[uuid.UUID]*domain.Permission),
		RefreshTokens: make(map[string]*domain.RefreshToken),
		AuditLogs:     make([]domain.AuditLog, 0),
	}

	// Seed standard permissions
	standardPerms := []struct {
		res, act, desc string
	}{
		{"user", "create", "Create users"},
		{"user", "read", "Read users"},
		{"user", "update", "Update users"},
		{"user", "delete", "Delete users"},
		{"role", "create", "Create roles"},
		{"role", "read", "Read roles"},
		{"role", "update", "Update roles"},
		{"role", "delete", "Delete roles"},
		{"role", "assign", "Assign roles"},
		{"tenant", "read", "Read tenant"},
		{"tenant", "update", "Update tenant"},
		{"audit", "read", "Read audit logs"},
	}

	for _, p := range standardPerms {
		id := uuid.New()
		store.Permissions[id] = &domain.Permission{
			ID:          id,
			Resource:    p.res,
			Action:      p.act,
			Description: p.desc,
			CreatedAt:   time.Now(),
		}
	}

	return store
}

// ----------------- Mock Tenant Repo -----------------
func (m *MockStore) Create(ctx context.Context, t *domain.Tenant) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, existing := range m.Tenants {
		if strings.EqualFold(existing.Code, t.Code) {
			return domain.ErrConflict
		}
	}

	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	t.CreatedAt = time.Now()
	t.UpdatedAt = time.Now()
	m.Tenants[t.ID] = t
	return nil
}

func (m *MockStore) GetByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	t, ok := m.Tenants[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	copy := *t
	return &copy, nil
}

func (m *MockStore) GetByCode(ctx context.Context, code string) (*domain.Tenant, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, t := range m.Tenants {
		if strings.EqualFold(t.Code, code) {
			copy := *t
			return &copy, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (m *MockStore) Update(ctx context.Context, t *domain.Tenant) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.Tenants[t.ID]
	if !ok {
		return domain.ErrNotFound
	}
	existing.Name = t.Name
	existing.Status = t.Status
	existing.UpdatedAt = time.Now()
	return nil
}

func (m *MockStore) List(ctx context.Context, params domain.PaginationParams) ([]domain.Tenant, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var list []domain.Tenant
	for _, t := range m.Tenants {
		if params.Search != "" && !strings.Contains(strings.ToLower(t.Name), strings.ToLower(params.Search)) &&
			!strings.Contains(strings.ToLower(t.Code), strings.ToLower(params.Search)) {
			continue
		}
		if params.Status != "" && t.Status != params.Status {
			continue
		}
		list = append(list, *t)
	}

	return list, int64(len(list)), nil
}

// ----------------- Mock User Repo -----------------
func (m *MockStore) CreateUserInTenant(ctx context.Context, tenantID uuid.UUID, user *domain.User, roleIDs []uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if email exists
	var existingUser *domain.User
	for _, u := range m.Users {
		if strings.EqualFold(u.Email, user.Email) {
			existingUser = u
			break
		}
	}

	if existingUser == nil {
		if user.ID == uuid.Nil {
			user.ID = uuid.New()
		}
		user.CreatedAt = time.Now()
		user.UpdatedAt = time.Now()
		m.Users[user.ID] = user
	} else {
		user.ID = existingUser.ID
		user.CreatedAt = existingUser.CreatedAt
		user.UpdatedAt = existingUser.UpdatedAt
	}

	key := tenantID.String() + ":" + user.ID.String()
	tu, exists := m.TenantUsers[key]
	if exists && tu.DeletedAt == nil {
		return domain.ErrConflict
	}

	m.TenantUsers[key] = &domain.TenantUser{
		ID:        uuid.New(),
		TenantID:  tenantID,
		UserID:    user.ID,
		Status:    domain.UserStatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		DeletedAt: nil,
	}

	if len(roleIDs) > 0 {
		m.UserRoles[key] = roleIDs
	}

	user.TenantID = &tenantID
	status := domain.UserStatusActive
	user.TenantStatus = &status
	return nil
}

func (m *MockStore) GetUserByID(ctx context.Context, tenantID, userID uuid.UUID) (*domain.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := tenantID.String() + ":" + userID.String()
	tu, ok := m.TenantUsers[key]
	if !ok || tu.DeletedAt != nil {
		return nil, domain.ErrNotFound
	}

	u, ok := m.Users[userID]
	if !ok {
		return nil, domain.ErrNotFound
	}

	copy := *u
	copy.TenantID = &tenantID
	copy.TenantStatus = &tu.Status
	copy.DeletedAt = tu.DeletedAt
	return &copy, nil
}

func (m *MockStore) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, u := range m.Users {
		if strings.EqualFold(u.Email, email) {
			copy := *u
			return &copy, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (m *MockStore) UpdateUser(ctx context.Context, tenantID, userID uuid.UUID, firstName, lastName, status string) (*domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := tenantID.String() + ":" + userID.String()
	tu, ok := m.TenantUsers[key]
	if !ok || tu.DeletedAt != nil {
		return nil, domain.ErrNotFound
	}

	u, ok := m.Users[userID]
	if !ok {
		return nil, domain.ErrNotFound
	}

	u.FirstName = firstName
	u.LastName = lastName
	u.UpdatedAt = time.Now()
	if status != "" {
		tu.Status = status
		tu.UpdatedAt = time.Now()
	}

	copy := *u
	copy.TenantID = &tenantID
	copy.TenantStatus = &tu.Status
	return &copy, nil
}

func (m *MockStore) SoftDelete(ctx context.Context, tenantID, userID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := tenantID.String() + ":" + userID.String()
	tu, ok := m.TenantUsers[key]
	if !ok || tu.DeletedAt != nil {
		return domain.ErrNotFound
	}

	now := time.Now()
	tu.DeletedAt = &now
	tu.Status = domain.UserStatusSuspended
	return nil
}

func (m *MockStore) ListByTenant(ctx context.Context, tenantID uuid.UUID, params domain.PaginationParams) ([]domain.User, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var list []domain.User
	for key, tu := range m.TenantUsers {
		if !strings.HasPrefix(key, tenantID.String()+":") || tu.DeletedAt != nil {
			continue
		}
		u, ok := m.Users[tu.UserID]
		if !ok {
			continue
		}
		if params.Search != "" && !strings.Contains(strings.ToLower(u.Email), strings.ToLower(params.Search)) &&
			!strings.Contains(strings.ToLower(u.FirstName), strings.ToLower(params.Search)) &&
			!strings.Contains(strings.ToLower(u.LastName), strings.ToLower(params.Search)) {
			continue
		}
		if params.Status != "" && tu.Status != params.Status {
			continue
		}
		copy := *u
		copy.TenantID = &tenantID
		copy.TenantStatus = &tu.Status
		list = append(list, copy)
	}

	return list, int64(len(list)), nil
}

func (m *MockStore) GetUserRolesInTenant(ctx context.Context, tenantID, userID uuid.UUID) ([]domain.Role, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := tenantID.String() + ":" + userID.String()
	roleIDs := m.UserRoles[key]
	var roles []domain.Role
	for _, rid := range roleIDs {
		if r, ok := m.Roles[rid]; ok && r.TenantID == tenantID {
			roles = append(roles, *r)
		}
	}
	return roles, nil
}

func (m *MockStore) GetUserTenants(ctx context.Context, userID uuid.UUID) ([]domain.Tenant, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var list []domain.Tenant
	for _, tu := range m.TenantUsers {
		if tu.UserID == userID && tu.DeletedAt == nil && tu.Status == domain.UserStatusActive {
			if t, ok := m.Tenants[tu.TenantID]; ok && t.Status == domain.TenantStatusActive {
				list = append(list, *t)
			}
		}
	}
	return list, nil
}

func (m *MockStore) GetTenantUserMembership(ctx context.Context, tenantID, userID uuid.UUID) (*domain.TenantUser, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := tenantID.String() + ":" + userID.String()
	tu, ok := m.TenantUsers[key]
	if !ok {
		return nil, domain.ErrNotFound
	}
	copy := *tu
	return &copy, nil
}

// ----------------- Mock Role Repo -----------------
func (m *MockStore) CreateRole(ctx context.Context, role *domain.Role) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, existing := range m.Roles {
		if existing.TenantID == role.TenantID && strings.EqualFold(existing.Name, role.Name) {
			return domain.ErrConflict
		}
	}

	if role.ID == uuid.Nil {
		role.ID = uuid.New()
	}
	role.CreatedAt = time.Now()
	role.UpdatedAt = time.Now()
	m.Roles[role.ID] = role
	return nil
}

func (m *MockStore) GetRoleByID(ctx context.Context, tenantID, roleID uuid.UUID) (*domain.Role, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	r, ok := m.Roles[roleID]
	if !ok || r.TenantID != tenantID {
		return nil, domain.ErrNotFound // Cross tenant returns 404
	}

	copy := *r
	copy.Permissions, _ = m.GetRolePermissions(ctx, r.ID)
	return &copy, nil
}

func (m *MockStore) ListRolesByTenant(ctx context.Context, tenantID uuid.UUID, params domain.PaginationParams) ([]domain.Role, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var list []domain.Role
	for _, r := range m.Roles {
		if r.TenantID != tenantID {
			continue
		}
		if params.Search != "" && !strings.Contains(strings.ToLower(r.Name), strings.ToLower(params.Search)) {
			continue
		}
		copy := *r
		copy.Permissions, _ = m.GetRolePermissions(ctx, r.ID)
		list = append(list, copy)
	}

	return list, int64(len(list)), nil
}

func (m *MockStore) UpdateRole(ctx context.Context, tenantID, roleID uuid.UUID, name, description string) (*domain.Role, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	r, ok := m.Roles[roleID]
	if !ok || r.TenantID != tenantID {
		return nil, domain.ErrNotFound // Cross-tenant access returns 404
	}

	r.Name = name
	r.Description = description
	r.UpdatedAt = time.Now()
	copy := *r
	return &copy, nil
}

func (m *MockStore) DeleteRole(ctx context.Context, tenantID, roleID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	r, ok := m.Roles[roleID]
	if !ok || r.TenantID != tenantID {
		return domain.ErrNotFound // Cross-tenant returns 404
	}
	if r.IsSystem {
		return domain.ErrCannotDeleteAdmin
	}

	delete(m.Roles, roleID)
	delete(m.RolePerms, roleID)
	return nil
}

func (m *MockStore) AssignPermissions(ctx context.Context, tenantID, roleID uuid.UUID, permissionIDs []uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	r, ok := m.Roles[roleID]
	if !ok || r.TenantID != tenantID {
		return domain.ErrNotFound // Cross-tenant returns 404
	}

	m.RolePerms[roleID] = permissionIDs
	return nil
}

func (m *MockStore) GetRolePermissions(ctx context.Context, roleID uuid.UUID) ([]domain.Permission, error) {
	pids := m.RolePerms[roleID]
	var list []domain.Permission
	for _, pid := range pids {
		if p, ok := m.Permissions[pid]; ok {
			list = append(list, *p)
		}
	}
	return list, nil
}

func (m *MockStore) AssignUserRoles(ctx context.Context, tenantID, userID uuid.UUID, roleIDs []uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := tenantID.String() + ":" + userID.String()
	tu, ok := m.TenantUsers[key]
	if !ok || tu.DeletedAt != nil {
		return domain.ErrNotFound
	}

	for _, rid := range roleIDs {
		r, ok := m.Roles[rid]
		if !ok || r.TenantID != tenantID {
			return domain.ErrBadRequest
		}
	}

	m.UserRoles[key] = roleIDs
	return nil
}

func (m *MockStore) GetUserPermissions(ctx context.Context, tenantID, userID uuid.UUID) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := tenantID.String() + ":" + userID.String()
	roleIDs := m.UserRoles[key]

	permSet := make(map[string]struct{})
	for _, rid := range roleIDs {
		r, ok := m.Roles[rid]
		if !ok || r.TenantID != tenantID {
			continue
		}
		pids := m.RolePerms[rid]
		for _, pid := range pids {
			if p, ok := m.Permissions[pid]; ok {
				permSet[p.String()] = struct{}{}
			}
		}
	}

	var list []string
	for p := range permSet {
		list = append(list, p)
	}
	return list, nil
}

// ----------------- Mock Auth Repo -----------------
func (m *MockStore) StoreRefreshToken(ctx context.Context, token *domain.RefreshToken) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if token.ID == uuid.Nil {
		token.ID = uuid.New()
	}
	token.CreatedAt = time.Now()
	m.RefreshTokens[token.TokenHash] = token
	return nil
}

func (m *MockStore) GetRefreshToken(ctx context.Context, tokenHash string) (*domain.RefreshToken, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	t, ok := m.RefreshTokens[tokenHash]
	if !ok {
		return nil, domain.ErrNotFound
	}
	copy := *t
	return &copy, nil
}

func (m *MockStore) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if t, ok := m.RefreshTokens[tokenHash]; ok {
		t.Revoked = true
	}
	return nil
}

func (m *MockStore) RevokeAllUserTokens(ctx context.Context, tenantID, userID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, t := range m.RefreshTokens {
		if t.TenantID == tenantID && t.UserID == userID {
			t.Revoked = true
		}
	}
	return nil
}

// ----------------- Mock Audit Repo -----------------
func (m *MockStore) CreateAudit(ctx context.Context, log *domain.AuditLog) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if log.ID == uuid.Nil {
		log.ID = uuid.New()
	}
	log.CreatedAt = time.Now()
	m.AuditLogs = append(m.AuditLogs, *log)
	return nil
}

func (m *MockStore) ListAuditByTenant(ctx context.Context, tenantID uuid.UUID, params domain.PaginationParams) ([]domain.AuditLog, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var list []domain.AuditLog
	for _, l := range m.AuditLogs {
		if l.TenantID != nil && *l.TenantID == tenantID {
			if params.Search != "" && !strings.Contains(strings.ToLower(l.Action), strings.ToLower(params.Search)) {
				continue
			}
			list = append(list, l)
		}
	}
	return list, int64(len(list)), nil
}

// ----------------- Mock Permission Repo -----------------
func (m *MockStore) ListAllPermissions(ctx context.Context) ([]domain.Permission, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var list []domain.Permission
	for _, p := range m.Permissions {
		list = append(list, *p)
	}
	return list, nil
}


// Adapters to satisfy package interfaces cleanly:

type tenantRepoAdapter struct{ store *MockStore }
func (a *tenantRepoAdapter) Create(ctx context.Context, t *domain.Tenant) error { return a.store.Create(ctx, t) }
func (a *tenantRepoAdapter) GetByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) { return a.store.GetByID(ctx, id) }
func (a *tenantRepoAdapter) GetByCode(ctx context.Context, code string) (*domain.Tenant, error) { return a.store.GetByCode(ctx, code) }
func (a *tenantRepoAdapter) Update(ctx context.Context, t *domain.Tenant) error { return a.store.Update(ctx, t) }
func (a *tenantRepoAdapter) List(ctx context.Context, p domain.PaginationParams) ([]domain.Tenant, int64, error) { return a.store.List(ctx, p) }

type userRepoAdapter struct{ store *MockStore }
func (a *userRepoAdapter) CreateUserInTenant(ctx context.Context, tid uuid.UUID, u *domain.User, r []uuid.UUID) error { return a.store.CreateUserInTenant(ctx, tid, u, r) }
func (a *userRepoAdapter) GetByID(ctx context.Context, tid, uid uuid.UUID) (*domain.User, error) { return a.store.GetUserByID(ctx, tid, uid) }
func (a *userRepoAdapter) GetByEmail(ctx context.Context, email string) (*domain.User, error) { return a.store.GetByEmail(ctx, email) }
func (a *userRepoAdapter) Update(ctx context.Context, tid, uid uuid.UUID, f, l, s string) (*domain.User, error) { return a.store.UpdateUser(ctx, tid, uid, f, l, s) }
func (a *userRepoAdapter) SoftDelete(ctx context.Context, tid, uid uuid.UUID) error { return a.store.SoftDelete(ctx, tid, uid) }
func (a *userRepoAdapter) ListByTenant(ctx context.Context, tid uuid.UUID, p domain.PaginationParams) ([]domain.User, int64, error) { return a.store.ListByTenant(ctx, tid, p) }
func (a *userRepoAdapter) GetUserRolesInTenant(ctx context.Context, tid, uid uuid.UUID) ([]domain.Role, error) { return a.store.GetUserRolesInTenant(ctx, tid, uid) }
func (a *userRepoAdapter) GetUserTenants(ctx context.Context, uid uuid.UUID) ([]domain.Tenant, error) { return a.store.GetUserTenants(ctx, uid) }
func (a *userRepoAdapter) GetTenantUserMembership(ctx context.Context, tid, uid uuid.UUID) (*domain.TenantUser, error) { return a.store.GetTenantUserMembership(ctx, tid, uid) }

type roleRepoAdapter struct{ store *MockStore }
func (a *roleRepoAdapter) Create(ctx context.Context, r *domain.Role) error { return a.store.CreateRole(ctx, r) }
func (a *roleRepoAdapter) GetByID(ctx context.Context, tid, rid uuid.UUID) (*domain.Role, error) { return a.store.GetRoleByID(ctx, tid, rid) }
func (a *roleRepoAdapter) ListByTenant(ctx context.Context, tid uuid.UUID, p domain.PaginationParams) ([]domain.Role, int64, error) { return a.store.ListRolesByTenant(ctx, tid, p) }
func (a *roleRepoAdapter) Update(ctx context.Context, tid, rid uuid.UUID, n, d string) (*domain.Role, error) { return a.store.UpdateRole(ctx, tid, rid, n, d) }
func (a *roleRepoAdapter) Delete(ctx context.Context, tid, rid uuid.UUID) error { return a.store.DeleteRole(ctx, tid, rid) }
func (a *roleRepoAdapter) AssignPermissions(ctx context.Context, tid, rid uuid.UUID, p []uuid.UUID) error { return a.store.AssignPermissions(ctx, tid, rid, p) }
func (a *roleRepoAdapter) GetRolePermissions(ctx context.Context, rid uuid.UUID) ([]domain.Permission, error) { return a.store.GetRolePermissions(ctx, rid) }
func (a *roleRepoAdapter) AssignUserRoles(ctx context.Context, tid, uid uuid.UUID, r []uuid.UUID) error { return a.store.AssignUserRoles(ctx, tid, uid, r) }
func (a *roleRepoAdapter) GetUserRoles(ctx context.Context, tid, uid uuid.UUID) ([]domain.Role, error) { return a.store.GetUserRolesInTenant(ctx, tid, uid) }
func (a *roleRepoAdapter) GetUserPermissions(ctx context.Context, tid, uid uuid.UUID) ([]string, error) { return a.store.GetUserPermissions(ctx, tid, uid) }

type authRepoAdapter struct{ store *MockStore }
func (a *authRepoAdapter) StoreRefreshToken(ctx context.Context, t *domain.RefreshToken) error { return a.store.StoreRefreshToken(ctx, t) }
func (a *authRepoAdapter) GetRefreshToken(ctx context.Context, th string) (*domain.RefreshToken, error) { return a.store.GetRefreshToken(ctx, th) }
func (a *authRepoAdapter) RevokeRefreshToken(ctx context.Context, th string) error { return a.store.RevokeRefreshToken(ctx, th) }
func (a *authRepoAdapter) RevokeAllUserTokens(ctx context.Context, tid, uid uuid.UUID) error { return a.store.RevokeAllUserTokens(ctx, tid, uid) }

type auditRepoAdapter struct{ store *MockStore }
func (a *auditRepoAdapter) Create(ctx context.Context, l *domain.AuditLog) error { return a.store.CreateAudit(ctx, l) }
func (a *auditRepoAdapter) ListByTenant(ctx context.Context, tid uuid.UUID, p domain.PaginationParams) ([]domain.AuditLog, int64, error) { return a.store.ListAuditByTenant(ctx, tid, p) }

type permRepoAdapter struct{ store *MockStore }
func (a *permRepoAdapter) ListAll(ctx context.Context) ([]domain.Permission, error) { return a.store.ListAllPermissions(ctx) }

func (m *MockStore) TenantRepo() *tenantRepoAdapter { return &tenantRepoAdapter{store: m} }
func (m *MockStore) UserRepo() *userRepoAdapter { return &userRepoAdapter{store: m} }
func (m *MockStore) RoleRepo() *roleRepoAdapter { return &roleRepoAdapter{store: m} }
func (m *MockStore) AuthRepo() *authRepoAdapter { return &authRepoAdapter{store: m} }
func (m *MockStore) AuditRepo() *auditRepoAdapter { return &auditRepoAdapter{store: m} }
func (m *MockStore) PermRepo() *permRepoAdapter { return &permRepoAdapter{store: m} }
