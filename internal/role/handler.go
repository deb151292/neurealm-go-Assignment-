package role

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

func (h *Handler) CreateRole(c *gin.Context) {
	tenantID, ok := domain.GetTenantID(c.Request.Context())
	if !ok {
		response.FromError(c, domain.ErrUnauthorized)
		return
	}

	var req CreateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_JSON", "malformed request payload", nil)
		return
	}

	role, err := h.service.CreateRole(c.Request.Context(), tenantID, req)
	if err != nil {
		response.FromError(c, err)
		return
	}

	response.Success(c, http.StatusCreated, "role created successfully", role)
}

func (h *Handler) GetRole(c *gin.Context) {
	tenantID, ok := domain.GetTenantID(c.Request.Context())
	if !ok {
		response.FromError(c, domain.ErrUnauthorized)
		return
	}

	roleID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "invalid role uuid format", nil)
		return
	}

	role, err := h.service.GetRole(c.Request.Context(), tenantID, roleID)
	if err != nil {
		response.FromError(c, err)
		return
	}

	response.Success(c, http.StatusOK, "role retrieved successfully", role)
}

func (h *Handler) ListRoles(c *gin.Context) {
	tenantID, ok := domain.GetTenantID(c.Request.Context())
	if !ok {
		response.FromError(c, domain.ErrUnauthorized)
		return
	}

	page, _ := strconv.Atoi(c.Query("page"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	search := c.Query("search")

	params := domain.PaginationParams{
		Page:   page,
		Limit:  limit,
		Search: search,
	}

	roles, total, err := h.service.ListRoles(c.Request.Context(), tenantID, params)
	if err != nil {
		response.FromError(c, err)
		return
	}

	response.Paginated(c, params.Page, params.Limit, total, roles)
}

func (h *Handler) UpdateRole(c *gin.Context) {
	tenantID, ok := domain.GetTenantID(c.Request.Context())
	if !ok {
		response.FromError(c, domain.ErrUnauthorized)
		return
	}

	roleID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "invalid role uuid format", nil)
		return
	}

	var req UpdateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_JSON", "malformed request payload", nil)
		return
	}

	role, err := h.service.UpdateRole(c.Request.Context(), tenantID, roleID, req)
	if err != nil {
		response.FromError(c, err)
		return
	}

	response.Success(c, http.StatusOK, "role updated successfully", role)
}

func (h *Handler) DeleteRole(c *gin.Context) {
	tenantID, ok := domain.GetTenantID(c.Request.Context())
	if !ok {
		response.FromError(c, domain.ErrUnauthorized)
		return
	}

	roleID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "invalid role uuid format", nil)
		return
	}

	if err := h.service.DeleteRole(c.Request.Context(), tenantID, roleID); err != nil {
		response.FromError(c, err)
		return
	}

	response.Success(c, http.StatusOK, "role deleted successfully", nil)
}

func (h *Handler) AssignPermissions(c *gin.Context) {
	tenantID, ok := domain.GetTenantID(c.Request.Context())
	if !ok {
		response.FromError(c, domain.ErrUnauthorized)
		return
	}

	roleID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "invalid role uuid format", nil)
		return
	}

	var req AssignPermissionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_JSON", "malformed request payload", nil)
		return
	}

	if err := h.service.AssignPermissions(c.Request.Context(), tenantID, roleID, req.PermissionIDs); err != nil {
		response.FromError(c, err)
		return
	}

	response.Success(c, http.StatusOK, "permissions assigned to role successfully", nil)
}
