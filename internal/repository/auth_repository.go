package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/deb151292/neurealm-go-Assignment-/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuthRepository interface {
	StoreRefreshToken(ctx context.Context, token *domain.RefreshToken) error
	GetRefreshToken(ctx context.Context, tokenHash string) (*domain.RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, tokenHash string) error
	RevokeAllUserTokens(ctx context.Context, tenantID, userID uuid.UUID) error
}

type pgxAuthRepository struct {
	pool *pgxpool.Pool
}

func NewAuthRepository(pool *pgxpool.Pool) AuthRepository {
	return &pgxAuthRepository{pool: pool}
}

func (r *pgxAuthRepository) StoreRefreshToken(ctx context.Context, token *domain.RefreshToken) error {
	query := `
		INSERT INTO refresh_tokens (token_hash, user_id, tenant_id, revoked, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		RETURNING id, created_at
	`
	return r.pool.QueryRow(ctx, query,
		token.TokenHash,
		token.UserID,
		token.TenantID,
		token.Revoked,
		token.ExpiresAt,
	).Scan(&token.ID, &token.CreatedAt)
}

func (r *pgxAuthRepository) GetRefreshToken(ctx context.Context, tokenHash string) (*domain.RefreshToken, error) {
	query := `
		SELECT id, token_hash, user_id, tenant_id, revoked, expires_at, created_at
		FROM refresh_tokens
		WHERE token_hash = $1
	`
	var t domain.RefreshToken
	err := r.pool.QueryRow(ctx, query, tokenHash).Scan(
		&t.ID,
		&t.TokenHash,
		&t.UserID,
		&t.TenantID,
		&t.Revoked,
		&t.ExpiresAt,
		&t.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get refresh token: %w", err)
	}
	return &t, nil
}

func (r *pgxAuthRepository) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	query := `UPDATE refresh_tokens SET revoked = TRUE WHERE token_hash = $1`
	_, err := r.pool.Exec(ctx, query, tokenHash)
	return err
}

func (r *pgxAuthRepository) RevokeAllUserTokens(ctx context.Context, tenantID, userID uuid.UUID) error {
	query := `UPDATE refresh_tokens SET revoked = TRUE WHERE tenant_id = $1 AND user_id = $2`
	_, err := r.pool.Exec(ctx, query, tenantID, userID)
	return err
}
