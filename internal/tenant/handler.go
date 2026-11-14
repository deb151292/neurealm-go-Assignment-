package tenant

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

func (h *Handler) CreateTenant(w http.ResponseWriter, r *http.Request) {
	var req CreateTenantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_JSON", "malformed request payload", nil)
		return
	}

	t, err := h.service.CreateTenant(r.Context(), req)
	if err != nil {
		response.FromError(w, err)
		return
	}

	response.Success(w, http.StatusCreated, "tenant created successfully", t)
}

func (h *Handler) GetTenant(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	var tenantID uuid.UUID
	var err error

	if idParam == "current" || idParam == "me" {
		var ok bool
		tenantID, ok = domain.GetTenantID(r.Context())
		if !ok {
			response.FromError(w, domain.ErrUnauthorized)
			return
		}
	} else {
		tenantID, err = uuid.Parse(idParam)
		if err != nil {
			response.Error(w, http.StatusBadRequest, "INVALID_ID", "invalid tenant uuid format", nil)
			return
		}

		// Enforce tenant boundary: non-platform admin can only access their own tenant
		tokenTenantID, ok := domain.GetTenantID(r.Context())
		isPlatformAdmin := domain.GetPlatformAdmin(r.Context())
		if ok && !isPlatformAdmin && tokenTenantID != tenantID {
			response.FromError(w, domain.ErrNotFound)
			return
		}
	}

	t, err := h.service.GetTenant(r.Context(), tenantID)
	if err != nil {
		response.FromError(w, err)
		return
	}

	response.Success(w, http.StatusOK, "tenant retrieved successfully", t)
}

func (h *Handler) UpdateTenant(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	tenantID, err := uuid.Parse(idParam)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_ID", "invalid tenant uuid format", nil)
		return
	}

	// Enforce tenant boundary: non-platform admin can only update their own tenant
	tokenTenantID, ok := domain.GetTenantID(r.Context())
	isPlatformAdmin := domain.GetPlatformAdmin(r.Context())
	if ok && !isPlatformAdmin && tokenTenantID != tenantID {
		response.FromError(w, domain.ErrNotFound)
		return
	}

	var req UpdateTenantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_JSON", "malformed request payload", nil)
		return
	}

	t, err := h.service.UpdateTenant(r.Context(), tenantID, req)
	if err != nil {
		response.FromError(w, err)
		return
	}

	response.Success(w, http.StatusOK, "tenant updated successfully", t)
}

func (h *Handler) ListTenants(w http.ResponseWriter, r *http.Request) {
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

	tenants, total, err := h.service.ListTenants(r.Context(), params)
	if err != nil {
		response.FromError(w, err)
		return
	}

	response.Paginated(w, params.Page, params.Limit, total, tenants)
}
