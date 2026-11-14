package role

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/deb151292/neurealm-go-Assignment-/internal/domain"
	"github.com/deb151292/neurealm-go-Assignment-/internal/response"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) CreateRole(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := domain.GetTenantID(r.Context())
	if !ok {
		response.FromError(w, domain.ErrUnauthorized)
		return
	}

	var req CreateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_JSON", "malformed request payload", nil)
		return
	}

	role, err := h.service.CreateRole(r.Context(), tenantID, req)
	if err != nil {
		response.FromError(w, err)
		return
	}

	response.Success(w, http.StatusCreated, "role created successfully", role)
}

func (h *Handler) GetRole(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := domain.GetTenantID(r.Context())
	if !ok {
		response.FromError(w, domain.ErrUnauthorized)
		return
	}

	roleID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_ID", "invalid role uuid format", nil)
		return
	}

	role, err := h.service.GetRole(r.Context(), tenantID, roleID)
	if err != nil {
		response.FromError(w, err)
		return
	}

	response.Success(w, http.StatusOK, "role retrieved successfully", role)
}

func (h *Handler) ListRoles(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := domain.GetTenantID(r.Context())
	if !ok {
		response.FromError(w, domain.ErrUnauthorized)
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	search := r.URL.Query().Get("search")

	params := domain.PaginationParams{
		Page:   page,
		Limit:  limit,
		Search: search,
	}

	roles, total, err := h.service.ListRoles(r.Context(), tenantID, params)
	if err != nil {
		response.FromError(w, err)
		return
	}

	response.Paginated(w, params.Page, params.Limit, total, roles)
}

func (h *Handler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := domain.GetTenantID(r.Context())
	if !ok {
		response.FromError(w, domain.ErrUnauthorized)
		return
	}

	roleID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_ID", "invalid role uuid format", nil)
		return
	}

	var req UpdateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_JSON", "malformed request payload", nil)
		return
	}

	role, err := h.service.UpdateRole(r.Context(), tenantID, roleID, req)
	if err != nil {
		response.FromError(w, err)
		return
	}

	response.Success(w, http.StatusOK, "role updated successfully", role)
}

func (h *Handler) DeleteRole(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := domain.GetTenantID(r.Context())
	if !ok {
		response.FromError(w, domain.ErrUnauthorized)
		return
	}

	roleID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_ID", "invalid role uuid format", nil)
		return
	}

	if err := h.service.DeleteRole(r.Context(), tenantID, roleID); err != nil {
		response.FromError(w, err)
		return
	}

	response.Success(w, http.StatusOK, "role deleted successfully", nil)
}

func (h *Handler) AssignPermissions(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := domain.GetTenantID(r.Context())
	if !ok {
		response.FromError(w, domain.ErrUnauthorized)
		return
	}

	roleID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_ID", "invalid role uuid format", nil)
		return
	}

	var req AssignPermissionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_JSON", "malformed request payload", nil)
		return
	}

	if err := h.service.AssignPermissions(r.Context(), tenantID, roleID, req.PermissionIDs); err != nil {
		response.FromError(w, err)
		return
	}

	response.Success(w, http.StatusOK, "permissions assigned to role successfully", nil)
}
