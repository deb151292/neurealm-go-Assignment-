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

type RoleRepository interface {
	Create(ctx context.Context, role *domain.Role) error
	GetByID(ctx context.Context, tenantID, roleID uuid.UUID) (*domain.Role, error)
	ListByTenant(ctx context.Context, tenantID uuid.UUID, params domain.PaginationParams) ([]domain.Role, int64, error)
	Update(ctx context.Context, tenantID, roleID uuid.UUID, name, description string) (*domain.Role, error)
	Delete(ctx context.Context, tenantID, roleID uuid.UUID) error
	AssignPermissions(ctx context.Context, tenantID, roleID uuid.UUID, permissionIDs []uuid.UUID) error
	GetRolePermissions(ctx context.Context, roleID uuid.UUID) ([]domain.Permission, error)
	AssignUserRoles(ctx context.Context, tenantID, userID uuid.UUID, roleIDs []uuid.UUID) error
	GetUserRoles(ctx context.Context, tenantID, userID uuid.UUID) ([]domain.Role, error)
	GetUserPermissions(ctx context.Context, tenantID, userID uuid.UUID) ([]string, error)
}

type pgxRoleRepository struct {
	pool *pgxpool.Pool
}

func NewRoleRepository(pool *pgxpool.Pool) RoleRepository {
	return &pgxRoleRepository{pool: pool}
}

func (r *pgxRoleRepository) Create(ctx context.Context, role *domain.Role) error {
	query := `
		INSERT INTO roles (tenant_id, name, description, is_system, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`
	err := r.pool.QueryRow(ctx, query, role.TenantID, role.Name, role.Description, role.IsSystem).
		Scan(&role.ID, &role.CreatedAt, &role.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrConflict
		}
		return fmt.Errorf("failed to create role: %w", err)
	}
	return nil
}

func (r *pgxRoleRepository) GetByID(ctx context.Context, tenantID, roleID uuid.UUID) (*domain.Role, error) {
	query := `
		SELECT id, tenant_id, name, description, is_system, created_at, updated_at
		FROM roles
		WHERE id = $1 AND tenant_id = $2
	`
	var role domain.Role
	err := r.pool.QueryRow(ctx, query, roleID, tenantID).
		Scan(&role.ID, &role.TenantID, &role.Name, &role.Description, &role.IsSystem, &role.CreatedAt, &role.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get role: %w", err)
	}

	perms, err := r.GetRolePermissions(ctx, role.ID)
	if err != nil {
		return nil, err
	}
	role.Permissions = perms

	return &role, nil
}

func (r *pgxRoleRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID, params domain.PaginationParams) ([]domain.Role, int64, error) {
	countQuery := `
		SELECT COUNT(*)
		FROM roles
		WHERE tenant_id = $1
		  AND ($2 = '' OR name ILIKE '%' || $2 || '%')
	`
	var total int64
	err := r.pool.QueryRow(ctx, countQuery, tenantID, params.Search).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count roles: %w", err)
	}

	query := `
		SELECT id, tenant_id, name, description, is_system, created_at, updated_at
		FROM roles
		WHERE tenant_id = $1
		  AND ($2 = '' OR name ILIKE '%' || $2 || '%')
		ORDER BY created_at ASC
		LIMIT $3 OFFSET $4
	`
	rows, err := r.pool.Query(ctx, query, tenantID, params.Search, params.Limit, params.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list roles: %w", err)
	}
	defer rows.Close()

	var roles []domain.Role
	for rows.Next() {
		var role domain.Role
		if err := rows.Scan(&role.ID, &role.TenantID, &role.Name, &role.Description, &role.IsSystem, &role.CreatedAt, &role.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("failed to scan role: %w", err)
		}
		roles = append(roles, role)
	}

	for i := range roles {
		perms, err := r.GetRolePermissions(ctx, roles[i].ID)
		if err == nil {
			roles[i].Permissions = perms
		}
	}

	return roles, total, nil
}

func (r *pgxRoleRepository) Update(ctx context.Context, tenantID, roleID uuid.UUID, name, description string) (*domain.Role, error) {
	query := `
		UPDATE roles
		SET name = $1, description = $2, updated_at = NOW()
		WHERE id = $3 AND tenant_id = $4
		RETURNING id, tenant_id, name, description, is_system, created_at, updated_at
	`
	var role domain.Role
	err := r.pool.QueryRow(ctx, query, name, description, roleID, tenantID).
		Scan(&role.ID, &role.TenantID, &role.Name, &role.Description, &role.IsSystem, &role.CreatedAt, &role.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, domain.ErrConflict
		}
		return nil, fmt.Errorf("failed to update role: %w", err)
	}

	perms, _ := r.GetRolePermissions(ctx, role.ID)
	role.Permissions = perms
	return &role, nil
}

func (r *pgxRoleRepository) Delete(ctx context.Context, tenantID, roleID uuid.UUID) error {
	var isSystem bool
	err := r.pool.QueryRow(ctx, `SELECT is_system FROM roles WHERE id = $1 AND tenant_id = $2`, roleID, tenantID).Scan(&isSystem)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		return err
	}
	if isSystem {
		return domain.ErrCannotDeleteAdmin
	}

	cmd, err := r.pool.Exec(ctx, `DELETE FROM roles WHERE id = $1 AND tenant_id = $2`, roleID, tenantID)
	if err != nil {
		return fmt.Errorf("failed to delete role: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}

func (r *pgxRoleRepository) AssignPermissions(ctx context.Context, tenantID, roleID uuid.UUID, permissionIDs []uuid.UUID) error {
	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM roles WHERE id = $1 AND tenant_id = $2)`, roleID, tenantID).Scan(&exists)
	if err != nil || !exists {
		return domain.ErrNotFound
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, `DELETE FROM role_permissions WHERE role_id = $1`, roleID)
	if err != nil {
		return fmt.Errorf("failed to clear existing permissions: %w", err)
	}

	insertQuery := `INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	for _, pid := range permissionIDs {
		_, err = tx.Exec(ctx, insertQuery, roleID, pid)
		if err != nil {
			return fmt.Errorf("failed to insert role permission: %w", err)
		}
	}

	return tx.Commit(ctx)
}

func (r *pgxRoleRepository) GetRolePermissions(ctx context.Context, roleID uuid.UUID) ([]domain.Permission, error) {
	query := `
		SELECT p.id, p.resource, p.action, p.description, p.created_at
		FROM permissions p
		INNER JOIN role_permissions rp ON p.id = rp.permission_id
		WHERE rp.role_id = $1
		ORDER BY p.resource, p.action
	`
	rows, err := r.pool.Query(ctx, query, roleID)
	if err != nil {
		return nil, fmt.Errorf("failed to query role permissions: %w", err)
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

func (r *pgxRoleRepository) AssignUserRoles(ctx context.Context, tenantID, userID uuid.UUID, roleIDs []uuid.UUID) error {
	var isMember bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM tenant_users WHERE tenant_id = $1 AND user_id = $2 AND deleted_at IS NULL)`, tenantID, userID).Scan(&isMember)
	if err != nil || !isMember {
		return domain.ErrNotFound
	}

	if len(roleIDs) > 0 {
		var count int
		err = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM roles WHERE tenant_id = $1 AND id = ANY($2)`, tenantID, roleIDs).Scan(&count)
		if err != nil || count != len(roleIDs) {
			return domain.ErrBadRequest
		}
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, `DELETE FROM user_roles WHERE tenant_id = $1 AND user_id = $2`, tenantID, userID)
	if err != nil {
		return fmt.Errorf("failed to clear user roles: %w", err)
	}

	insertQuery := `INSERT INTO user_roles (tenant_id, user_id, role_id, created_at) VALUES ($1, $2, $3, NOW()) ON CONFLICT DO NOTHING`
	for _, rid := range roleIDs {
		_, err = tx.Exec(ctx, insertQuery, tenantID, userID, rid)
		if err != nil {
			return fmt.Errorf("failed to assign user role: %w", err)
		}
	}

	return tx.Commit(ctx)
}

func (r *pgxRoleRepository) GetUserRoles(ctx context.Context, tenantID, userID uuid.UUID) ([]domain.Role, error) {
	query := `
		SELECT r.id, r.tenant_id, r.name, r.description, r.is_system, r.created_at, r.updated_at
		FROM roles r
		INNER JOIN user_roles ur ON r.id = ur.role_id
		WHERE ur.tenant_id = $1 AND ur.user_id = $2
		ORDER BY r.name ASC
	`
	rows, err := r.pool.Query(ctx, query, tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user roles: %w", err)
	}
	defer rows.Close()

	var roles []domain.Role
	for rows.Next() {
		var role domain.Role
		if err := rows.Scan(&role.ID, &role.TenantID, &role.Name, &role.Description, &role.IsSystem, &role.CreatedAt, &role.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan role: %w", err)
		}
		roles = append(roles, role)
	}
	return roles, nil
}

func (r *pgxRoleRepository) GetUserPermissions(ctx context.Context, tenantID, userID uuid.UUID) ([]string, error) {
	query := `
		SELECT DISTINCT p.resource || ':' || p.action
		FROM permissions p
		INNER JOIN role_permissions rp ON p.id = rp.permission_id
		INNER JOIN user_roles ur ON rp.role_id = ur.role_id
		WHERE ur.tenant_id = $1 AND ur.user_id = $2
	`
	rows, err := r.pool.Query(ctx, query, tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query effective user permissions: %w", err)
	}
	defer rows.Close()

	var perms []string
	for rows.Next() {
		var perm string
		if err := rows.Scan(&perm); err != nil {
			return nil, fmt.Errorf("failed to scan permission: %w", err)
		}
		perms = append(perms, perm)
	}
	return perms, nil
}
