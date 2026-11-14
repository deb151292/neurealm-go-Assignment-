package permission

import (
	"context"
	"net/http"

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

func (h *Handler) ListPermissions(w http.ResponseWriter, r *http.Request) {
	perms, err := h.service.ListPermissions(r.Context())
	if err != nil {
		response.FromError(w, err)
		return
	}

	response.Success(w, http.StatusOK, "permissions retrieved successfully", perms)
}
