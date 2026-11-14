package user

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/neurealm/multi-tenant-system/internal/domain"
	"github.com/neurealm/multi-tenant-system/internal/response"
	"github.com/neurealm/multi-tenant-system/internal/role"
)

type Handler struct {
	service     *Service
	roleService *role.Service
}

func NewHandler(service *Service, roleService *role.Service) *Handler {
	return &Handler{
		service:     service,
		roleService: roleService,
	}
}

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	// Strictly enforces tenant isolation: tenant_id taken from JWT token only!
	tenantID, ok := domain.GetTenantID(r.Context())
	if !ok {
		response.FromError(w, domain.ErrUnauthorized)
		return
	}

	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_JSON", "malformed request payload", nil)
		return
	}

	user, err := h.service.CreateUser(r.Context(), tenantID, req)
	if err != nil {
		response.FromError(w, err)
		return
	}

	response.Success(w, http.StatusCreated, "user created successfully", user)
}

func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	// Strictly enforces tenant isolation: tenant_id taken from JWT token only!
	tenantID, ok := domain.GetTenantID(r.Context())
	if !ok {
		response.FromError(w, domain.ErrUnauthorized)
		return
	}

	targetUserID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_ID", "invalid user uuid format", nil)
		return
	}

	// Cross-tenant access check: querying another tenant's user returns 404
	user, err := h.service.GetUser(r.Context(), tenantID, targetUserID)
	if err != nil {
		response.FromError(w, err)
		return
	}

	response.Success(w, http.StatusOK, "user retrieved successfully", user)
}

func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := domain.GetTenantID(r.Context())
	if !ok {
		response.FromError(w, domain.ErrUnauthorized)
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	search := r.URL.Query().Get("search")
	status := r.URL.Query().Get("status")

	params := domain.PaginationParams{
		Page:   page,
		Limit:  limit,
		Search: search,
		Status: status,
	}

	users, total, err := h.service.ListUsers(r.Context(), tenantID, params)
	if err != nil {
		response.FromError(w, err)
		return
	}

	response.Paginated(w, params.Page, params.Limit, total, users)
}

func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := domain.GetTenantID(r.Context())
	if !ok {
		response.FromError(w, domain.ErrUnauthorized)
		return
	}

	targetUserID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_ID", "invalid user uuid format", nil)
		return
	}

	var req UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_JSON", "malformed request payload", nil)
		return
	}

	// Cross-tenant access returns 404
	user, err := h.service.UpdateUser(r.Context(), tenantID, targetUserID, req)
	if err != nil {
		response.FromError(w, err)
		return
	}

	response.Success(w, http.StatusOK, "user updated successfully", user)
}

func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := domain.GetTenantID(r.Context())
	if !ok {
		response.FromError(w, domain.ErrUnauthorized)
		return
	}

	targetUserID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_ID", "invalid user uuid format", nil)
		return
	}

	// Cross-tenant access returns 404
	if err := h.service.SoftDeleteUser(r.Context(), tenantID, targetUserID); err != nil {
		response.FromError(w, err)
		return
	}

	response.Success(w, http.StatusOK, "user soft-deleted successfully", nil)
}

func (h *Handler) AssignRoles(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := domain.GetTenantID(r.Context())
	if !ok {
		response.FromError(w, domain.ErrUnauthorized)
		return
	}

	targetUserID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_ID", "invalid user uuid format", nil)
		return
	}

	var req role.AssignUserRolesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_JSON", "malformed request payload", nil)
		return
	}

	if err := h.roleService.AssignUserRoles(r.Context(), tenantID, targetUserID, req.RoleIDs); err != nil {
		response.FromError(w, err)
		return
	}

	response.Success(w, http.StatusOK, "roles assigned to user successfully", nil)
}
