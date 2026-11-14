package handler

import (
	"net/http"

	"github.com/deb151292/neurealm-go-Assignment-/internal/response"
	"github.com/deb151292/neurealm-go-Assignment-/internal/service"
	"github.com/gin-gonic/gin"
)

type PermissionHandler struct {
	service *service.PermissionService
}

func NewPermissionHandler(service *service.PermissionService) *PermissionHandler {
	return &PermissionHandler{service: service}
}

func (h *PermissionHandler) ListPermissions(c *gin.Context) {
	perms, err := h.service.ListPermissions(c.Request.Context())
	if err != nil {
		response.FromError(c, err)
		return
	}

	response.Success(c, http.StatusOK, "permissions retrieved successfully", perms)
}
