package service

import (
	"context"
	"strings"
	"time"

	"github.com/deb151292/neurealm-go-Assignment-/internal/auth"
	"github.com/deb151292/neurealm-go-Assignment-/internal/cache"
	"github.com/deb151292/neurealm-go-Assignment-/internal/domain"
	"github.com/deb151292/neurealm-go-Assignment-/internal/repository"
	"github.com/deb151292/neurealm-go-Assignment-/internal/validator"
	"github.com/google/uuid"
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

type AuthService struct {
	authRepo     repository.AuthRepository
	userRepo     repository.UserRepository
	tenantRepo   repository.TenantRepository
	roleRepo     repository.RoleRepository
	jwtService   *auth.JWTService
	cache        *cache.PermissionCache
	auditService *AuditService
}

func NewAuthService(
	authRepo repository.AuthRepository,
	userRepo repository.UserRepository,
	tenantRepo repository.TenantRepository,
	roleRepo repository.RoleRepository,
	jwtService *auth.JWTService,
	cache *cache.PermissionCache,
	auditService *AuditService,
) *AuthService {
	return &AuthService{
		authRepo:     authRepo,
		userRepo:     userRepo,
		tenantRepo:   tenantRepo,
		roleRepo:     roleRepo,
		jwtService:   jwtService,
		cache:        cache,
		auditService: auditService,
	}
}

func (s *AuthService) Login(ctx context.Context, req LoginRequest) (*MultiTenantLoginResult, error) {
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

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
		return nil, domain.ErrUnauthorized
	}

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

		tu, err := s.userRepo.GetTenantUserMembership(ctx, t.ID, u.ID)
		if err != nil || tu.DeletedAt != nil {
			return nil, domain.ErrUnauthorized
		}
		if tu.Status == domain.UserStatusSuspended {
			return nil, domain.ErrUserSuspended
		}
		targetTenant = t
	} else {
		tenants, err := s.userRepo.GetUserTenants(ctx, u.ID)
		if err != nil || len(tenants) == 0 {
			return nil, domain.ErrUnauthorized
		}

		if len(tenants) > 1 {
			return &MultiTenantLoginResult{
				RequiresTenantSelection: true,
				AvailableTenants:        tenants,
			}, nil
		}
		targetTenant = &tenants[0]
	}

	tokens, err := s.issueTokens(ctx, u, targetTenant)
	if err != nil {
		return nil, err
	}

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

func (s *AuthService) Refresh(ctx context.Context, rawRefreshToken string) (*AuthTokens, error) {
	if strings.TrimSpace(rawRefreshToken) == "" {
		return nil, domain.ErrInvalidToken
	}

	tokenHash := s.jwtService.HashRefreshToken(rawRefreshToken)
	storedToken, err := s.authRepo.GetRefreshToken(ctx, tokenHash)
	if err != nil || storedToken.Revoked || time.Now().After(storedToken.ExpiresAt) {
		return nil, domain.ErrInvalidToken
	}

	_ = s.authRepo.RevokeRefreshToken(ctx, tokenHash)

	t, err := s.tenantRepo.GetByID(ctx, storedToken.TenantID)
	if err != nil || t.Status == domain.TenantStatusSuspended {
		return nil, domain.ErrTenantSuspended
	}

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

func (s *AuthService) SwitchTenant(ctx context.Context, userID uuid.UUID, newTenantCode string) (*AuthTokens, error) {
	code := strings.ToLower(strings.TrimSpace(newTenantCode))
	t, err := s.tenantRepo.GetByCode(ctx, code)
	if err != nil {
		return nil, domain.ErrNotFound
	}
	if t.Status == domain.TenantStatusSuspended {
		return nil, domain.ErrTenantSuspended
	}

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

func (s *AuthService) Logout(ctx context.Context, rawRefreshToken string) error {
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

func (s *AuthService) issueTokens(ctx context.Context, u *domain.User, t *domain.Tenant) (*AuthTokens, error) {
	rolesList, err := s.roleRepo.GetUserRoles(ctx, t.ID, u.ID)
	if err != nil {
		return nil, err
	}
	roleNames := make([]string, 0, len(rolesList))
	for _, r := range rolesList {
		roleNames = append(roleNames, r.Name)
	}

	perms, err := s.roleRepo.GetUserPermissions(ctx, t.ID, u.ID)
	if err != nil {
		return nil, err
	}

	if s.cache != nil {
		s.cache.Set(t.ID, u.ID, perms)
	}

	accessToken, err := s.jwtService.GenerateAccessToken(u, t, roleNames, perms)
	if err != nil {
		return nil, err
	}

	rawRefreshToken, tokenHash, err := s.jwtService.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

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
		ExpiresIn:    s.jwtService.AccessExpirySeconds(),
		User:         u,
		Tenant:       t,
		Roles:        roleNames,
		Permissions:  perms,
	}, nil
}
