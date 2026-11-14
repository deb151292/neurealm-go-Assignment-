package auth

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/neurealm/multi-tenant-system/internal/audit"
	"github.com/neurealm/multi-tenant-system/internal/cache"
	"github.com/neurealm/multi-tenant-system/internal/domain"
	"github.com/neurealm/multi-tenant-system/internal/role"
	"github.com/neurealm/multi-tenant-system/internal/tenant"
	"github.com/neurealm/multi-tenant-system/internal/user"
	"github.com/neurealm/multi-tenant-system/internal/validator"
	"golang.org/x/crypto/bcrypt"
)

type LoginRequest struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	TenantCode string `json:"tenant_code,omitempty"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type SwitchTenantRequest struct {
	TenantCode string `json:"tenant_code"`
}

type AuthTokens struct {
	AccessToken  string         `json:"access_token"`
	RefreshToken string         `json:"refresh_token"`
	TokenType    string         `json:"token_type"`
	ExpiresIn    int64          `json:"expires_in"` // seconds
	User         *domain.User   `json:"user"`
	Tenant       *domain.Tenant `json:"tenant"`
	Roles        []string       `json:"roles"`
	Permissions  []string       `json:"permissions"`
}

type MultiTenantLoginResult struct {
	RequiresTenantSelection bool            `json:"requires_tenant_selection"`
	AvailableTenants        []domain.Tenant `json:"available_tenants,omitempty"`
	Tokens                  *AuthTokens     `json:"tokens,omitempty"`
}

type Service struct {
	authRepo     Repository
	userRepo     user.Repository
	tenantRepo   tenant.Repository
	roleRepo     role.Repository
	jwtService   *JWTService
	cache        *cache.PermissionCache
	auditService *audit.Service
}

func NewService(
	authRepo Repository,
	userRepo user.Repository,
	tenantRepo tenant.Repository,
	roleRepo role.Repository,
	jwtService *JWTService,
	cache *cache.PermissionCache,
	auditService *audit.Service,
) *Service {
	return &Service{
		authRepo:     authRepo,
		userRepo:     userRepo,
		tenantRepo:   tenantRepo,
		roleRepo:     roleRepo,
		jwtService:   jwtService,
		cache:        cache,
		auditService: auditService,
	}
}

func (s *Service) Login(ctx context.Context, req LoginRequest) (*MultiTenantLoginResult, error) {
	v := validator.New()
	v.Email(req.Email, "email")
	v.RequiredString(req.Password, "password")
	if !v.Valid() {
		return nil, v.ToAppError()
	}

	normalizedEmail := strings.ToLower(strings.TrimSpace(req.Email))
	u, err := s.userRepo.GetByEmail(ctx, normalizedEmail)
	if err != nil {
		return nil, domain.ErrUnauthorized
	}

	// Verify bcrypt password hash
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
		return nil, domain.ErrUnauthorized
	}

	// Determine Tenant scope
	var targetTenant *domain.Tenant
	if req.TenantCode != "" {
		code := strings.ToLower(strings.TrimSpace(req.TenantCode))
		t, err := s.tenantRepo.GetByCode(ctx, code)
		if err != nil {
			return nil, domain.ErrNotFound
		}
		if t.Status == domain.TenantStatusSuspended {
			return nil, domain.ErrTenantSuspended
		}

		// Verify user membership in this tenant
		tu, err := s.userRepo.GetTenantUserMembership(ctx, t.ID, u.ID)
		if err != nil || tu.DeletedAt != nil {
			return nil, domain.ErrUnauthorized
		}
		if tu.Status == domain.UserStatusSuspended {
			return nil, domain.ErrUserSuspended
		}
		targetTenant = t
	} else {
		// Multi-tenant check: fetch all tenants for this user
		tenants, err := s.userRepo.GetUserTenants(ctx, u.ID)
		if err != nil || len(tenants) == 0 {
			return nil, domain.ErrUnauthorized
		}

		if len(tenants) > 1 {
			// User belongs to multiple tenants, return tenant selection prompt
			return &MultiTenantLoginResult{
				RequiresTenantSelection: true,
				AvailableTenants:        tenants,
			}, nil
		}
		// Exactly one active tenant
		targetTenant = &tenants[0]
	}

	tokens, err := s.issueTokens(ctx, u, targetTenant)
	if err != nil {
		return nil, err
	}

	// Record audit event
	if s.auditService != nil {
		s.auditService.RecordAsync(&domain.AuditLog{
			TenantID:   &targetTenant.ID,
			ActorID:    &u.ID,
			ActorEmail: u.Email,
			Action:     "auth:login",
			TargetType: "session",
			TargetID:   u.ID.String(),
			Metadata: map[string]interface{}{
				"tenant_code": targetTenant.Code,
			},
		})
	}

	return &MultiTenantLoginResult{
		RequiresTenantSelection: false,
		Tokens:                  tokens,
	}, nil
}

func (s *Service) Refresh(ctx context.Context, rawRefreshToken string) (*AuthTokens, error) {
	if strings.TrimSpace(rawRefreshToken) == "" {
		return nil, domain.ErrInvalidToken
	}

	tokenHash := s.jwtService.HashRefreshToken(rawRefreshToken)
	storedToken, err := s.authRepo.GetRefreshToken(ctx, tokenHash)
	if err != nil || storedToken.Revoked || time.Now().After(storedToken.ExpiresAt) {
		return nil, domain.ErrInvalidToken
	}

	// Token rotation: Revoke old refresh token immediately
	_ = s.authRepo.RevokeRefreshToken(ctx, tokenHash)

	// Fetch tenant and verify status
	t, err := s.tenantRepo.GetByID(ctx, storedToken.TenantID)
	if err != nil || t.Status == domain.TenantStatusSuspended {
		return nil, domain.ErrTenantSuspended
	}

	// Fetch user in tenant
	u, err := s.userRepo.GetByID(ctx, t.ID, storedToken.UserID)
	if err != nil || u.DeletedAt != nil {
		return nil, domain.ErrUnauthorized
	}
	if u.TenantStatus != nil && *u.TenantStatus == domain.UserStatusSuspended {
		return nil, domain.ErrUserSuspended
	}

	tokens, err := s.issueTokens(ctx, u, t)
	if err != nil {
		return nil, err
	}

	if s.auditService != nil {
		s.auditService.RecordAsync(&domain.AuditLog{
			TenantID:   &t.ID,
			ActorID:    &u.ID,
			ActorEmail: u.Email,
			Action:     "auth:refresh",
			TargetType: "session",
			TargetID:   u.ID.String(),
			Metadata: map[string]interface{}{
				"tenant_code": t.Code,
			},
		})
	}

	return tokens, nil
}

func (s *Service) SwitchTenant(ctx context.Context, userID uuid.UUID, newTenantCode string) (*AuthTokens, error) {
	code := strings.ToLower(strings.TrimSpace(newTenantCode))
	t, err := s.tenantRepo.GetByCode(ctx, code)
	if err != nil {
		return nil, domain.ErrNotFound
	}
	if t.Status == domain.TenantStatusSuspended {
		return nil, domain.ErrTenantSuspended
	}

	// Verify membership in new tenant
	tu, err := s.userRepo.GetTenantUserMembership(ctx, t.ID, userID)
	if err != nil || tu.DeletedAt != nil {
		return nil, domain.ErrUnauthorized
	}
	if tu.Status == domain.UserStatusSuspended {
		return nil, domain.ErrUserSuspended
	}

	u, err := s.userRepo.GetByID(ctx, t.ID, userID)
	if err != nil {
		return nil, err
	}

	tokens, err := s.issueTokens(ctx, u, t)
	if err != nil {
		return nil, err
	}

	if s.auditService != nil {
		s.auditService.RecordAsync(&domain.AuditLog{
			TenantID:   &t.ID,
			ActorID:    &u.ID,
			ActorEmail: u.Email,
			Action:     "auth:switch_tenant",
			TargetType: "tenant",
			TargetID:   t.ID.String(),
			Metadata: map[string]interface{}{
				"new_tenant_code": t.Code,
			},
		})
	}

	return tokens, nil
}

func (s *Service) Logout(ctx context.Context, rawRefreshToken string) error {
	if rawRefreshToken != "" {
		tokenHash := s.jwtService.HashRefreshToken(rawRefreshToken)
		_ = s.authRepo.RevokeRefreshToken(ctx, tokenHash)
	}

	userID, ok := domain.GetUserID(ctx)
	tenantID, tok := domain.GetTenantID(ctx)
	if ok && tok {
		if s.cache != nil {
			s.cache.InvalidateUser(tenantID, userID)
		}
		actorEmail := domain.GetUserEmail(ctx)
		if s.auditService != nil {
			s.auditService.RecordAsync(&domain.AuditLog{
				TenantID:   &tenantID,
				ActorID:    &userID,
				ActorEmail: actorEmail,
				Action:     "auth:logout",
				TargetType: "session",
				TargetID:   userID.String(),
				Metadata:   map[string]interface{}{},
			})
		}
	}

	return nil
}

func (s *Service) issueTokens(ctx context.Context, u *domain.User, t *domain.Tenant) (*AuthTokens, error) {
	// Fetch roles
	rolesList, err := s.roleRepo.GetUserRoles(ctx, t.ID, u.ID)
	if err != nil {
		return nil, err
	}
	roleNames := make([]string, 0, len(rolesList))
	for _, r := range rolesList {
		roleNames = append(roleNames, r.Name)
	}

	// Fetch permissions
	perms, err := s.roleRepo.GetUserPermissions(ctx, t.ID, u.ID)
	if err != nil {
		return nil, err
	}

	// Cache permissions in fast memory
	if s.cache != nil {
		s.cache.Set(t.ID, u.ID, perms)
	}

	// Generate access token
	accessToken, err := s.jwtService.GenerateAccessToken(u, t, roleNames, perms)
	if err != nil {
		return nil, err
	}

	// Generate refresh token
	rawRefreshToken, tokenHash, err := s.jwtService.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

	// Store refresh token
	rfModel := &domain.RefreshToken{
		TokenHash: tokenHash,
		UserID:    u.ID,
		TenantID:  t.ID,
		Revoked:   false,
		ExpiresAt: time.Now().Add(s.jwtService.RefreshExpiry()),
	}
	if err := s.authRepo.StoreRefreshToken(ctx, rfModel); err != nil {
		return nil, err
	}

	return &AuthTokens{
		AccessToken:  accessToken,
		RefreshToken: rawRefreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(s.jwtService.accessExpiry.Seconds()),
		User:         u,
		Tenant:       t,
		Roles:        roleNames,
		Permissions:  perms,
	}, nil
}
