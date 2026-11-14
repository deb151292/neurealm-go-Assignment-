package audit

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/neurealm/multi-tenant-system/internal/domain"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Record(ctx context.Context, entry *domain.AuditLog) error {
	return s.repo.Create(ctx, entry)
}

func (s *Service) RecordAsync(entry *domain.AuditLog) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.repo.Create(ctx, entry); err != nil {
			log.Printf("[Audit] Error recording audit log: %v", err)
		}
	}()
}

func (s *Service) ListByTenant(ctx context.Context, tenantID uuid.UUID, params domain.PaginationParams) ([]domain.AuditLog, int64, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.Limit < 1 || params.Limit > 100 {
		params.Limit = 20
	}
	return s.repo.ListByTenant(ctx, tenantID, params)
}
