package auth

import (
	"encoding/json"
	"net/http"

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

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_JSON", "malformed request payload", nil)
		return
	}

	result, err := h.service.Login(r.Context(), req)
	if err != nil {
		response.FromError(w, err)
		return
	}

	if result.RequiresTenantSelection {
		response.Success(w, http.StatusOK, "please select a tenant to proceed", result)
		return
	}

	response.Success(w, http.StatusOK, "login successful", result.Tokens)
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req RefreshTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_JSON", "malformed request payload", nil)
		return
	}

	tokens, err := h.service.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		response.FromError(w, err)
		return
	}

	response.Success(w, http.StatusOK, "token refreshed successfully", tokens)
}

func (h *Handler) SwitchTenant(w http.ResponseWriter, r *http.Request) {
	userID, ok := domain.GetUserID(r.Context())
	if !ok {
		response.FromError(w, domain.ErrUnauthorized)
		return
	}

	var req SwitchTenantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_JSON", "malformed request payload", nil)
		return
	}

	tokens, err := h.service.SwitchTenant(r.Context(), userID, req.TenantCode)
	if err != nil {
		response.FromError(w, err)
		return
	}

	response.Success(w, http.StatusOK, "tenant switched successfully", tokens)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var req RefreshTokenRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	if err := h.service.Logout(r.Context(), req.RefreshToken); err != nil {
		response.FromError(w, err)
		return
	}

	response.Success(w, http.StatusOK, "logged out successfully", nil)
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := domain.GetUserID(r.Context())
	tenantID, tok := domain.GetTenantID(r.Context())
	if !ok || !tok {
		response.FromError(w, domain.ErrUnauthorized)
		return
	}

	currentUser, err := h.userService.GetUser(r.Context(), tenantID, userID)
	if err != nil {
		response.FromError(w, err)
		return
	}

	currentTenant, err := h.tenantRepo.GetByID(r.Context(), tenantID)
	if err != nil {
		response.FromError(w, err)
		return
	}

	roles, _ := h.roleService.GetUserRoles(r.Context(), tenantID, userID)
	perms, _ := h.roleService.GetUserPermissions(r.Context(), tenantID, userID)

	profile := map[string]interface{}{
		"user":        currentUser,
		"tenant":      currentTenant,
		"roles":       roles,
		"permissions": perms,
	}

	response.Success(w, http.StatusOK, "profile retrieved successfully", profile)
}
