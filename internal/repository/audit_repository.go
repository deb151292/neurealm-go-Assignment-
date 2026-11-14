package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/deb151292/neurealm-go-Assignment-/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuditRepository interface {
	Create(ctx context.Context, log *domain.AuditLog) error
	ListByTenant(ctx context.Context, tenantID uuid.UUID, params domain.PaginationParams) ([]domain.AuditLog, int64, error)
}

type pgxAuditRepository struct {
	pool *pgxpool.Pool
}

func NewAuditRepository(pool *pgxpool.Pool) AuditRepository {
	return &pgxAuditRepository{pool: pool}
}

func (r *pgxAuditRepository) Create(ctx context.Context, log *domain.AuditLog) error {
	metaJSON, err := json.Marshal(log.Metadata)
	if err != nil {
		metaJSON = []byte("{}")
	}

	query := `
		INSERT INTO audit_logs (tenant_id, actor_id, actor_email, action, target_type, target_id, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
		RETURNING id, created_at
	`
	return r.pool.QueryRow(ctx, query,
		log.TenantID,
		log.ActorID,
		log.ActorEmail,
		log.Action,
		log.TargetType,
		log.TargetID,
		metaJSON,
	).Scan(&log.ID, &log.CreatedAt)
}

func (r *pgxAuditRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID, params domain.PaginationParams) ([]domain.AuditLog, int64, error) {
	countQuery := `
		SELECT COUNT(*)
		FROM audit_logs
		WHERE tenant_id = $1
		  AND ($2 = '' OR action ILIKE '%' || $2 || '%')
	`
	var total int64
	err := r.pool.QueryRow(ctx, countQuery, tenantID, params.Search).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count audit logs: %w", err)
	}

	query := `
		SELECT id, tenant_id, actor_id, actor_email, action, target_type, target_id, metadata, created_at
		FROM audit_logs
		WHERE tenant_id = $1
		  AND ($2 = '' OR action ILIKE '%' || $2 || '%')
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`
	rows, err := r.pool.Query(ctx, query, tenantID, params.Search, params.Limit, params.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query audit logs: %w", err)
	}
	defer rows.Close()

	var logs []domain.AuditLog
	for rows.Next() {
		var l domain.AuditLog
		var metaBytes []byte
		if err := rows.Scan(
			&l.ID,
			&l.TenantID,
			&l.ActorID,
			&l.ActorEmail,
			&l.Action,
			&l.TargetType,
			&l.TargetID,
			&metaBytes,
			&l.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan audit log: %w", err)
		}
		if len(metaBytes) > 0 {
			_ = json.Unmarshal(metaBytes, &l.Metadata)
		}
		logs = append(logs, l)
	}

	return logs, total, nil
}
