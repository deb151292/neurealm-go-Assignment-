package handler

import (
	"net/http"
	"strconv"

	"github.com/deb151292/neurealm-go-Assignment-/internal/domain"
	"github.com/deb151292/neurealm-go-Assignment-/internal/response"
	"github.com/deb151292/neurealm-go-Assignment-/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type UserHandler struct {
	service     *service.UserService
	roleService *service.RoleService
}

func NewUserHandler(service *service.UserService, roleService *service.RoleService) *UserHandler {
	return &UserHandler{
		service:     service,
		roleService: roleService,
	}
}

func (h *UserHandler) CreateUser(c *gin.Context) {
	tenantID, ok := domain.GetTenantID(c.Request.Context())
	if !ok {
		response.FromError(c, domain.ErrUnauthorized)
		return
	}

	var req service.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_JSON", "malformed request payload", nil)
		return
	}

	user, err := h.service.CreateUser(c.Request.Context(), tenantID, req)
	if err != nil {
		response.FromError(c, err)
		return
	}

	response.Success(c, http.StatusCreated, "user created successfully", user)
}

func (h *UserHandler) GetUser(c *gin.Context) {
	tenantID, ok := domain.GetTenantID(c.Request.Context())
	if !ok {
		response.FromError(c, domain.ErrUnauthorized)
		return
	}

	targetUserID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "invalid user uuid format", nil)
		return
	}

	user, err := h.service.GetUser(c.Request.Context(), tenantID, targetUserID)
	if err != nil {
		response.FromError(c, err)
		return
	}

	response.Success(c, http.StatusOK, "user retrieved successfully", user)
}

func (h *UserHandler) ListUsers(c *gin.Context) {
	tenantID, ok := domain.GetTenantID(c.Request.Context())
	if !ok {
		response.FromError(c, domain.ErrUnauthorized)
		return
	}

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

	users, total, err := h.service.ListUsers(c.Request.Context(), tenantID, params)
	if err != nil {
		response.FromError(c, err)
		return
	}

	response.Paginated(c, params.Page, params.Limit, total, users)
}

func (h *UserHandler) UpdateUser(c *gin.Context) {
	tenantID, ok := domain.GetTenantID(c.Request.Context())
	if !ok {
		response.FromError(c, domain.ErrUnauthorized)
		return
	}

	targetUserID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "invalid user uuid format", nil)
		return
	}

	var req service.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_JSON", "malformed request payload", nil)
		return
	}

	user, err := h.service.UpdateUser(c.Request.Context(), tenantID, targetUserID, req)
	if err != nil {
		response.FromError(c, err)
		return
	}

	response.Success(c, http.StatusOK, "user updated successfully", user)
}

func (h *UserHandler) DeleteUser(c *gin.Context) {
	tenantID, ok := domain.GetTenantID(c.Request.Context())
	if !ok {
		response.FromError(c, domain.ErrUnauthorized)
		return
	}

	targetUserID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "invalid user uuid format", nil)
		return
	}

	if err := h.service.SoftDeleteUser(c.Request.Context(), tenantID, targetUserID); err != nil {
		response.FromError(c, err)
		return
	}

	response.Success(c, http.StatusOK, "user soft-deleted successfully", nil)
}

func (h *UserHandler) AssignRoles(c *gin.Context) {
	tenantID, ok := domain.GetTenantID(c.Request.Context())
	if !ok {
		response.FromError(c, domain.ErrUnauthorized)
		return
	}

	targetUserID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "invalid user uuid format", nil)
		return
	}

	var req service.AssignUserRolesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_JSON", "malformed request payload", nil)
		return
	}

	if err := h.roleService.AssignUserRoles(c.Request.Context(), tenantID, targetUserID, req.RoleIDs); err != nil {
		response.FromError(c, err)
		return
	}

	response.Success(c, http.StatusOK, "roles assigned to user successfully", nil)
}
