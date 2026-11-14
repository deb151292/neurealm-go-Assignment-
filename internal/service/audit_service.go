package service

import (
	"context"
	"log"
	"time"

	"github.com/deb151292/neurealm-go-Assignment-/internal/domain"
	"github.com/deb151292/neurealm-go-Assignment-/internal/repository"
	"github.com/google/uuid"
)

type AuditService struct {
	repo repository.AuditRepository
}

func NewAuditService(repo repository.AuditRepository) *AuditService {
	return &AuditService{repo: repo}
}

func (s *AuditService) Record(ctx context.Context, entry *domain.AuditLog) error {
	return s.repo.Create(ctx, entry)
}

func (s *AuditService) RecordAsync(entry *domain.AuditLog) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.repo.Create(ctx, entry); err != nil {
			log.Printf("[Audit] Error recording audit log: %v", err)
		}
	}()
}

func (s *AuditService) ListByTenant(ctx context.Context, tenantID uuid.UUID, params domain.PaginationParams) ([]domain.AuditLog, int64, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.Limit < 1 || params.Limit > 100 {
		params.Limit = 20
	}
	return s.repo.ListByTenant(ctx, tenantID, params)
}
