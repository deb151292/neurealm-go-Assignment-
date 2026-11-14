package repository

import (
	"context"
	"fmt"

	"github.com/deb151292/neurealm-go-Assignment-/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PermissionRepository interface {
	ListAll(ctx context.Context) ([]domain.Permission, error)
}

type pgxPermissionRepository struct {
	pool *pgxpool.Pool
}

func NewPermissionRepository(pool *pgxpool.Pool) PermissionRepository {
	return &pgxPermissionRepository{pool: pool}
}

func (r *pgxPermissionRepository) ListAll(ctx context.Context) ([]domain.Permission, error) {
	query := `
		SELECT id, resource, action, description, created_at
		FROM permissions
		ORDER BY resource ASC, action ASC
	`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query permissions: %w", err)
	}
	defer rows.Close()

	var perms []domain.Permission
	for rows.Next() {
		var p domain.Permission
		if err := rows.Scan(&p.ID, &p.Resource, &p.Action, &p.Description, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan permission: %w", err)
		}
		perms = append(perms, p)
	}

	return perms, nil
}
