package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/deb151292/neurealm-go-Assignment-/internal/domain"
	"github.com/deb151292/neurealm-go-Assignment-/internal/response"
	"github.com/deb151292/neurealm-go-Assignment-/internal/role"
	"github.com/deb151292/neurealm-go-Assignment-/internal/tenant"
	"github.com/deb151292/neurealm-go-Assignment-/internal/user"
)

type Handler struct {
	service     *Service
	userService *user.Service
	tenantRepo  tenant.Repository
	roleService *role.Service
}

func NewHandler(service *Service, userService *user.Service, tenantRepo tenant.Repository, roleService *role.Service) *Handler {
	return &Handler{
		service:     service,
		userService: userService,
		tenantRepo:  tenantRepo,
		roleService: roleService,
	}
}

func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_JSON", "malformed request payload", nil)
		return
	}

	result, err := h.service.Login(c.Request.Context(), req)
	if err != nil {
		response.FromError(c, err)
		return
	}

	if result.RequiresTenantSelection {
		response.Success(c, http.StatusOK, "please select a tenant to proceed", result)
		return
	}

	response.Success(c, http.StatusOK, "login successful", result.Tokens)
}

func (h *Handler) Refresh(c *gin.Context) {
	var req RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_JSON", "malformed request payload", nil)
		return
	}

	tokens, err := h.service.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		response.FromError(c, err)
		return
	}

	response.Success(c, http.StatusOK, "token refreshed successfully", tokens)
}

func (h *Handler) SwitchTenant(c *gin.Context) {
	userID, ok := domain.GetUserID(c.Request.Context())
	if !ok {
		response.FromError(c, domain.ErrUnauthorized)
		return
	}

	var req SwitchTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_JSON", "malformed request payload", nil)
		return
	}

	tokens, err := h.service.SwitchTenant(c.Request.Context(), userID, req.TenantCode)
	if err != nil {
		response.FromError(c, err)
		return
	}

	response.Success(c, http.StatusOK, "tenant switched successfully", tokens)
}

func (h *Handler) Logout(c *gin.Context) {
	var req RefreshTokenRequest
	_ = c.ShouldBindJSON(&req)

	if err := h.service.Logout(c.Request.Context(), req.RefreshToken); err != nil {
		response.FromError(c, err)
		return
	}

	response.Success(c, http.StatusOK, "logged out successfully", nil)
}

func (h *Handler) Me(c *gin.Context) {
	ctx := c.Request.Context()
	userID, ok := domain.GetUserID(ctx)
	tenantID, tok := domain.GetTenantID(ctx)
	if !ok || !tok {
		response.FromError(c, domain.ErrUnauthorized)
		return
	}

	currentUser, err := h.userService.GetUser(ctx, tenantID, userID)
	if err != nil {
		response.FromError(c, err)
		return
	}

	currentTenant, err := h.tenantRepo.GetByID(ctx, tenantID)
	if err != nil {
		response.FromError(c, err)
		return
	}

	roles, _ := h.roleService.GetUserRoles(ctx, tenantID, userID)
	perms, _ := h.roleService.GetUserPermissions(ctx, tenantID, userID)

	profile := map[string]interface{}{
		"user":        currentUser,
		"tenant":      currentTenant,
		"roles":       roles,
		"permissions": perms,
	}

	response.Success(c, http.StatusOK, "profile retrieved successfully", profile)
}
