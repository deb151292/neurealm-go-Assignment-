package permission

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/deb151292/neurealm-go-Assignment-/internal/domain"
	"github.com/deb151292/neurealm-go-Assignment-/internal/response"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListPermissions(ctx context.Context) ([]domain.Permission, error) {
	return s.repo.ListAll(ctx)
}

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) ListPermissions(c *gin.Context) {
	perms, err := h.service.ListPermissions(c.Request.Context())
	if err != nil {
		response.FromError(c, err)
		return
	}

	response.Success(c, http.StatusOK, "permissions retrieved successfully", perms)
}
