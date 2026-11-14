package user

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/deb151292/neurealm-go-Assignment-/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	CreateUserInTenant(ctx context.Context, tenantID uuid.UUID, user *domain.User, roleIDs []uuid.UUID) error
	GetByID(ctx context.Context, tenantID, userID uuid.UUID) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	Update(ctx context.Context, tenantID, userID uuid.UUID, firstName, lastName, status string) (*domain.User, error)
	SoftDelete(ctx context.Context, tenantID, userID uuid.UUID) error
	ListByTenant(ctx context.Context, tenantID uuid.UUID, params domain.PaginationParams) ([]domain.User, int64, error)
	GetUserRolesInTenant(ctx context.Context, tenantID, userID uuid.UUID) ([]domain.Role, error)
	GetUserTenants(ctx context.Context, userID uuid.UUID) ([]domain.Tenant, error)
	GetTenantUserMembership(ctx context.Context, tenantID, userID uuid.UUID) (*domain.TenantUser, error)
}

type pgxRepository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgxRepository{pool: pool}
}

func (r *pgxRepository) CreateUserInTenant(ctx context.Context, tenantID uuid.UUID, user *domain.User, roleIDs []uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// 1. Check if user with email already exists globally
	var existingID uuid.UUID
	err = tx.QueryRow(ctx, `SELECT id FROM users WHERE email = $1`, user.Email).Scan(&existingID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("failed to check existing user: %w", err)
	}

	if errors.Is(err, pgx.ErrNoRows) {
		// Insert new global user
		queryUser := `
			INSERT INTO users (email, password_hash, first_name, last_name, is_platform_admin, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
			RETURNING id, created_at, updated_at
		`
		err = tx.QueryRow(ctx, queryUser, user.Email, user.PasswordHash, user.FirstName, user.LastName, user.IsPlatformAdmin).
			Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
		if err != nil {
			return fmt.Errorf("failed to create user: %w", err)
		}
	} else {
		// Existing user found
		user.ID = existingID
		// Check if already an active member of this tenant
		var isMember bool
		var deletedAt *time.Time
		err = tx.QueryRow(ctx, `SELECT TRUE, deleted_at FROM tenant_users WHERE tenant_id = $1 AND user_id = $2`, tenantID, user.ID).Scan(&isMember, &deletedAt)
		if err == nil {
			if deletedAt == nil {
				return domain.ErrConflict // User already exists in this tenant
			}
			// Re-activate soft-deleted user membership
			_, err = tx.Exec(ctx, `UPDATE tenant_users SET deleted_at = NULL, status = 'active', updated_at = NOW() WHERE tenant_id = $1 AND user_id = $2`, tenantID, user.ID)
			if err != nil {
				return fmt.Errorf("failed to reactivate tenant user: %w", err)
			}
		}
	}

	// 2. Link user to tenant in tenant_users
	queryTenantUser := `
		INSERT INTO tenant_users (tenant_id, user_id, status, created_at, updated_at)
		VALUES ($1, $2, 'active', NOW(), NOW())
		ON CONFLICT (tenant_id, user_id) DO UPDATE SET deleted_at = NULL, status = 'active', updated_at = NOW()
	`
	if _, err := tx.Exec(ctx, queryTenantUser, tenantID, user.ID); err != nil {
		return fmt.Errorf("failed to link user to tenant: %w", err)
	}

	// 3. Assign initial roles
	if len(roleIDs) > 0 {
		for _, rid := range roleIDs {
			_, err = tx.Exec(ctx, `
				INSERT INTO user_roles (tenant_id, user_id, role_id, created_at)
				VALUES ($1, $2, $3, NOW())
				ON CONFLICT DO NOTHING
			`, tenantID, user.ID, rid)
			if err != nil {
				return fmt.Errorf("failed to assign initial role: %w", err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit user creation: %w", err)
	}

	user.TenantID = &tenantID
	status := domain.UserStatusActive
	user.TenantStatus = &status
	return nil
}

func (r *pgxRepository) GetByID(ctx context.Context, tenantID, userID uuid.UUID) (*domain.User, error) {
	// Strictly enforces tenant isolation: cross-tenant access returns ErrNotFound (404)
	query := `
		SELECT u.id, u.email, u.first_name, u.last_name, u.is_platform_admin, u.created_at, u.updated_at,
		       tu.tenant_id, tu.status, tu.deleted_at
		FROM users u
		INNER JOIN tenant_users tu ON u.id = tu.user_id
		WHERE u.id = $1 AND tu.tenant_id = $2 AND tu.deleted_at IS NULL
	`
	var user domain.User
	var tid uuid.UUID
	var status string
	var deletedAt *time.Time

	err := r.pool.QueryRow(ctx, query, userID, tenantID).Scan(
		&user.ID,
		&user.Email,
		&user.FirstName,
		&user.LastName,
		&user.IsPlatformAdmin,
		&user.CreatedAt,
		&user.UpdatedAt,
		&tid,
		&status,
		&deletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	user.TenantID = &tid
	user.TenantStatus = &status
	user.DeletedAt = deletedAt

	// Populate tenant roles
	roles, err := r.GetUserRolesInTenant(ctx, tenantID, user.ID)
	if err == nil {
		user.Roles = roles
	}

	return &user, nil
}

func (r *pgxRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `
		SELECT id, email, password_hash, first_name, last_name, is_platform_admin, created_at, updated_at
		FROM users
		WHERE email = $1
	`
	var user domain.User
	err := r.pool.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.FirstName,
		&user.LastName,
		&user.IsPlatformAdmin,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}
	return &user, nil
}

func (r *pgxRepository) Update(ctx context.Context, tenantID, userID uuid.UUID, firstName, lastName, status string) (*domain.User, error) {
	// Strictly enforces tenant isolation: cross-tenant access returns ErrNotFound (404)
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Verify membership in this tenant
	var exists bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM tenant_users WHERE tenant_id = $1 AND user_id = $2 AND deleted_at IS NULL)`, tenantID, userID).Scan(&exists)
	if err != nil || !exists {
		return nil, domain.ErrNotFound
	}

	// Update user details
	_, err = tx.Exec(ctx, `UPDATE users SET first_name = $1, last_name = $2, updated_at = NOW() WHERE id = $3`, firstName, lastName, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to update user names: %w", err)
	}

	// Update tenant membership status if provided
	if status != "" {
		_, err = tx.Exec(ctx, `UPDATE tenant_users SET status = $1, updated_at = NOW() WHERE tenant_id = $2 AND user_id = $3`, status, tenantID, userID)
		if err != nil {
			return nil, fmt.Errorf("failed to update user tenant status: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return r.GetByID(ctx, tenantID, userID)
}

func (r *pgxRepository) SoftDelete(ctx context.Context, tenantID, userID uuid.UUID) error {
	// Soft delete from tenant_users table: cross-tenant access returns ErrNotFound (404)
	query := `
		UPDATE tenant_users
		SET deleted_at = NOW(), status = 'suspended', updated_at = NOW()
		WHERE tenant_id = $1 AND user_id = $2 AND deleted_at IS NULL
	`
	cmd, err := r.pool.Exec(ctx, query, tenantID, userID)
	if err != nil {
		return fmt.Errorf("failed to soft delete user: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}

func (r *pgxRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID, params domain.PaginationParams) ([]domain.User, int64, error) {
	countQuery := `
		SELECT COUNT(*)
		FROM users u
		INNER JOIN tenant_users tu ON u.id = tu.user_id
		WHERE tu.tenant_id = $1 AND tu.deleted_at IS NULL
		  AND ($2 = '' OR u.email ILIKE '%' || $2 || '%' OR u.first_name ILIKE '%' || $2 || '%' OR u.last_name ILIKE '%' || $2 || '%')
		  AND ($3 = '' OR tu.status = $3)
	`
	var total int64
	err := r.pool.QueryRow(ctx, countQuery, tenantID, params.Search, params.Status).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count users in tenant: %w", err)
	}

	query := `
		SELECT u.id, u.email, u.first_name, u.last_name, u.is_platform_admin, u.created_at, u.updated_at,
		       tu.tenant_id, tu.status, tu.deleted_at
		FROM users u
		INNER JOIN tenant_users tu ON u.id = tu.user_id
		WHERE tu.tenant_id = $1 AND tu.deleted_at IS NULL
		  AND ($2 = '' OR u.email ILIKE '%' || $2 || '%' OR u.first_name ILIKE '%' || $2 || '%' OR u.last_name ILIKE '%' || $2 || '%')
		  AND ($3 = '' OR tu.status = $3)
		ORDER BY u.created_at DESC
		LIMIT $4 OFFSET $5
	`
	rows, err := r.pool.Query(ctx, query, tenantID, params.Search, params.Status, params.Limit, params.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	var users []domain.User
	for rows.Next() {
		var u domain.User
		var tid uuid.UUID
		var status string
		var deletedAt *time.Time
		if err := rows.Scan(
			&u.ID,
			&u.Email,
			&u.FirstName,
			&u.LastName,
			&u.IsPlatformAdmin,
			&u.CreatedAt,
			&u.UpdatedAt,
			&tid,
			&status,
			&deletedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan user: %w", err)
		}
		u.TenantID = &tid
		u.TenantStatus = &status
		u.DeletedAt = deletedAt
		users = append(users, u)
	}

	// Populate roles for each user
	for i := range users {
		roles, err := r.GetUserRolesInTenant(ctx, tenantID, users[i].ID)
		if err == nil {
			users[i].Roles = roles
		}
	}

	return users, total, nil
}

func (r *pgxRepository) GetUserRolesInTenant(ctx context.Context, tenantID, userID uuid.UUID) ([]domain.Role, error) {
	query := `
		SELECT r.id, r.tenant_id, r.name, r.description, r.is_system, r.created_at, r.updated_at
		FROM roles r
		INNER JOIN user_roles ur ON r.id = ur.role_id
		WHERE ur.tenant_id = $1 AND ur.user_id = $2
		ORDER BY r.name ASC
	`
	rows, err := r.pool.Query(ctx, query, tenantID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []domain.Role
	for rows.Next() {
		var r domain.Role
		if err := rows.Scan(&r.ID, &r.TenantID, &r.Name, &r.Description, &r.IsSystem, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		roles = append(roles, r)
	}
	return roles, nil
}

func (r *pgxRepository) GetUserTenants(ctx context.Context, userID uuid.UUID) ([]domain.Tenant, error) {
	query := `
		SELECT t.id, t.name, t.code, t.status, t.created_at, t.updated_at
		FROM tenants t
		INNER JOIN tenant_users tu ON t.id = tu.tenant_id
		WHERE tu.user_id = $1 AND tu.deleted_at IS NULL AND t.status = 'active' AND tu.status = 'active'
		ORDER BY t.name ASC
	`
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user tenants: %w", err)
	}
	defer rows.Close()

	var tenants []domain.Tenant
	for rows.Next() {
		var t domain.Tenant
		if err := rows.Scan(&t.ID, &t.Name, &t.Code, &t.Status, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tenants = append(tenants, t)
	}
	return tenants, nil
}

func (r *pgxRepository) GetTenantUserMembership(ctx context.Context, tenantID, userID uuid.UUID) (*domain.TenantUser, error) {
	query := `
		SELECT id, tenant_id, user_id, status, created_at, updated_at, deleted_at
		FROM tenant_users
		WHERE tenant_id = $1 AND user_id = $2
	`
	var tu domain.TenantUser
	err := r.pool.QueryRow(ctx, query, tenantID, userID).Scan(
		&tu.ID,
		&tu.TenantID,
		&tu.UserID,
		&tu.Status,
		&tu.CreatedAt,
		&tu.UpdatedAt,
		&tu.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &tu, nil
}
