package database

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Migrator struct {
	pool *pgxpool.Pool
}

func NewMigrator(pool *pgxpool.Pool) *Migrator {
	return &Migrator{pool: pool}
}

// RunMigrations executes a list of SQL migration files or raw SQL statements
func (m *Migrator) RunMigrationFile(ctx context.Context, filePath string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read migration file %s: %w", filePath, err)
	}

	return m.ExecSQL(ctx, string(content))
}

func (m *Migrator) ExecSQL(ctx context.Context, sqlScript string) error {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	_, err = tx.Exec(ctx, sqlScript)
	if err != nil {
		return fmt.Errorf("failed to execute migration sql: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit migration transaction: %w", err)
	}

	return nil
}

// AutoMigrate applies schema up migration
func (m *Migrator) AutoMigrate(ctx context.Context, migrationsDir string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	upFile := filepath.Join(migrationsDir, "000001_init_schema.up.sql")
	if _, err := os.Stat(upFile); err == nil {
		log.Printf("[Migrator] Applying schema migration: %s", upFile)
		if err := m.RunMigrationFile(ctx, upFile); err != nil {
			// If already created, don't fail if table exists error, but report
			if strings.Contains(err.Error(), "already exists") {
				log.Printf("[Migrator] Schema already exists, skipping")
				return nil
			}
			return err
		}
		log.Println("[Migrator] Successfully applied schema migrations")
	}

	return nil
}
