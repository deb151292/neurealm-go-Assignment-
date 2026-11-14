package service

import (
	"context"

	"github.com/deb151292/neurealm-go-Assignment-/internal/domain"
	"github.com/deb151292/neurealm-go-Assignment-/internal/repository"
)

type PermissionService struct {
	repo repository.PermissionRepository
}

func NewPermissionService(repo repository.PermissionRepository) *PermissionService {
	return &PermissionService{repo: repo}
}

func (s *PermissionService) ListPermissions(ctx context.Context) ([]domain.Permission, error) {
	return s.repo.ListAll(ctx)
}
