package tenant

import (
	"net/http"
	"strconv"

	"github.com/deb151292/neurealm-go-Assignment-/internal/domain"
	"github.com/deb151292/neurealm-go-Assignment-/internal/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) CreateTenant(c *gin.Context) {
	var req CreateTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_JSON", "malformed request payload", nil)
		return
	}

	t, err := h.service.CreateTenant(c.Request.Context(), req)
	if err != nil {
		response.FromError(c, err)
		return
	}

	response.Success(c, http.StatusCreated, "tenant created successfully", t)
}

func (h *Handler) GetTenant(c *gin.Context) {
	idParam := c.Param("id")
	var tenantID uuid.UUID
	var err error

	if idParam == "current" || idParam == "me" {
		var ok bool
		tenantID, ok = domain.GetTenantID(c.Request.Context())
		if !ok {
			response.FromError(c, domain.ErrUnauthorized)
			return
		}
	} else {
		tenantID, err = uuid.Parse(idParam)
		if err != nil {
			response.Error(c, http.StatusBadRequest, "INVALID_ID", "invalid tenant uuid format", nil)
			return
		}

		// Enforce tenant boundary: non-platform admin can only access their own tenant
		tokenTenantID, ok := domain.GetTenantID(c.Request.Context())
		isPlatformAdmin := domain.GetPlatformAdmin(c.Request.Context())
		if ok && !isPlatformAdmin && tokenTenantID != tenantID {
			response.FromError(c, domain.ErrNotFound)
			return
		}
	}

	t, err := h.service.GetTenant(c.Request.Context(), tenantID)
	if err != nil {
		response.FromError(c, err)
		return
	}

	response.Success(c, http.StatusOK, "tenant retrieved successfully", t)
}

func (h *Handler) UpdateTenant(c *gin.Context) {
	idParam := c.Param("id")
	tenantID, err := uuid.Parse(idParam)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "invalid tenant uuid format", nil)
		return
	}

	// Enforce tenant boundary: non-platform admin can only update their own tenant
	tokenTenantID, ok := domain.GetTenantID(c.Request.Context())
	isPlatformAdmin := domain.GetPlatformAdmin(c.Request.Context())
	if ok && !isPlatformAdmin && tokenTenantID != tenantID {
		response.FromError(c, domain.ErrNotFound)
		return
	}

	var req UpdateTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_JSON", "malformed request payload", nil)
		return
	}

	t, err := h.service.UpdateTenant(c.Request.Context(), tenantID, req)
	if err != nil {
		response.FromError(c, err)
		return
	}

	response.Success(c, http.StatusOK, "tenant updated successfully", t)
}

func (h *Handler) ListTenants(c *gin.Context) {
	page, _ := strconv.Atoi(c.Query("page"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	search := c.Query("search")
	status := c.Query("status")

	params := domain.PaginationParams{
		Page:   page,
		Limit:  limit,
		Search: search,
		Status: status,
	}

	tenants, total, err := h.service.ListTenants(c.Request.Context(), params)
	if err != nil {
		response.FromError(c, err)
		return
	}

	response.Paginated(c, params.Page, params.Limit, total, tenants)
}
