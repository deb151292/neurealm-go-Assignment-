package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/deb151292/neurealm-go-Assignment-/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TenantRepository interface {
	Create(ctx context.Context, t *domain.Tenant) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error)
	GetByCode(ctx context.Context, code string) (*domain.Tenant, error)
	Update(ctx context.Context, t *domain.Tenant) error
	List(ctx context.Context, params domain.PaginationParams) ([]domain.Tenant, int64, error)
}

type pgxTenantRepository struct {
	pool *pgxpool.Pool
}

func NewTenantRepository(pool *pgxpool.Pool) TenantRepository {
	return &pgxTenantRepository{pool: pool}
}

func (r *pgxTenantRepository) Create(ctx context.Context, t *domain.Tenant) error {
	query := `
		INSERT INTO tenants (name, code, status, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`
	err := r.pool.QueryRow(ctx, query, t.Name, t.Code, t.Status).Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrConflict
		}
		return fmt.Errorf("failed to insert tenant: %w", err)
	}
	return nil
}

func (r *pgxTenantRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	query := `
		SELECT id, name, code, status, created_at, updated_at
		FROM tenants
		WHERE id = $1
	`
	var t domain.Tenant
	err := r.pool.QueryRow(ctx, query, id).Scan(&t.ID, &t.Name, &t.Code, &t.Status, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get tenant: %w", err)
	}
	return &t, nil
}

func (r *pgxTenantRepository) GetByCode(ctx context.Context, code string) (*domain.Tenant, error) {
	query := `
		SELECT id, name, code, status, created_at, updated_at
		FROM tenants
		WHERE code = $1
	`
	var t domain.Tenant
	err := r.pool.QueryRow(ctx, query, code).Scan(&t.ID, &t.Name, &t.Code, &t.Status, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get tenant by code: %w", err)
	}
	return &t, nil
}

func (r *pgxTenantRepository) Update(ctx context.Context, t *domain.Tenant) error {
	query := `
		UPDATE tenants
		SET name = $1, status = $2, updated_at = NOW()
		WHERE id = $3
		RETURNING updated_at
	`
	err := r.pool.QueryRow(ctx, query, t.Name, t.Status, t.ID).Scan(&t.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		return fmt.Errorf("failed to update tenant: %w", err)
	}
	return nil
}

func (r *pgxTenantRepository) List(ctx context.Context, params domain.PaginationParams) ([]domain.Tenant, int64, error) {
	countQuery := `
		SELECT COUNT(*)
		FROM tenants
		WHERE ($1 = '' OR name ILIKE '%' || $1 || '%' OR code ILIKE '%' || $1 || '%')
		  AND ($2 = '' OR status = $2)
	`
	var total int64
	err := r.pool.QueryRow(ctx, countQuery, params.Search, params.Status).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count tenants: %w", err)
	}

	query := `
		SELECT id, name, code, status, created_at, updated_at
		FROM tenants
		WHERE ($1 = '' OR name ILIKE '%' || $1 || '%' OR code ILIKE '%' || $1 || '%')
		  AND ($2 = '' OR status = $2)
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`
	rows, err := r.pool.Query(ctx, query, params.Search, params.Status, params.Limit, params.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list tenants: %w", err)
	}
	defer rows.Close()

	var tenants []domain.Tenant
	for rows.Next() {
		var t domain.Tenant
		if err := rows.Scan(&t.ID, &t.Name, &t.Code, &t.Status, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("failed to scan tenant: %w", err)
		}
		tenants = append(tenants, t)
	}

	return tenants, total, nil
}
