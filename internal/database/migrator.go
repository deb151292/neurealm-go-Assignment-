package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
)

type Migrator struct {
	pool          *pgxpool.Pool
	migrationsDir string
}

func NewMigrator(pool *pgxpool.Pool, migrationsDir string) *Migrator {
	if migrationsDir == "" {
		migrationsDir = "./database/migrations"
	}
	return &Migrator{
		pool:          pool,
		migrationsDir: migrationsDir,
	}
}

func (m *Migrator) openSQLDB() (*sql.DB, error) {
	connConfig := m.pool.Config().ConnConfig
	connStr := connConfig.ConnString()

	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open sql.DB with pgx: %w", err)
	}
	return db, nil
}

func (m *Migrator) Up(ctx context.Context) error {
	db, err := m.openSQLDB()
	if err != nil {
		return err
	}
	defer db.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	log.Printf("[Goose] Running database migrations UP from: %s", m.migrationsDir)
	if err := goose.UpContext(ctx, db, m.migrationsDir); err != nil {
		return fmt.Errorf("goose up failed: %w", err)
	}

	log.Println("[Goose] Migrations applied successfully")
	return nil
}

func (m *Migrator) Down(ctx context.Context) error {
	db, err := m.openSQLDB()
	if err != nil {
		return err
	}
	defer db.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	log.Printf("[Goose] Rolling back database migration DOWN from: %s", m.migrationsDir)
	if err := goose.DownContext(ctx, db, m.migrationsDir); err != nil {
		return fmt.Errorf("goose down failed: %w", err)
	}

	log.Println("[Goose] Rollback completed successfully")
	return nil
}

func (m *Migrator) Status(ctx context.Context) error {
	db, err := m.openSQLDB()
	if err != nil {
		return err
	}
	defer db.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	return goose.StatusContext(ctx, db, m.migrationsDir)
}

func (m *Migrator) AutoMigrate(ctx context.Context, dir string) error {
	if dir != "" {
		m.migrationsDir = dir
	}
	return m.Up(ctx)
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

	return tx.Commit(ctx)
}
