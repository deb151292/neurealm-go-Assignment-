package main

import (
	"context"
	"log"
	"time"

	"github.com/deb151292/neurealm-go-Assignment-/internal/config"
	"github.com/deb151292/neurealm-go-Assignment-/internal/database"
)

func main() {
	log.Println("[Seed] Loading configuration and initializing database connection...")
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[Seed] Configuration error: %v", err)
	}

	db, err := database.NewPool(cfg)
	if err != nil {
		log.Fatalf("[Seed] Database connection error: %v", err)
	}
	defer db.Close()

	migrator := database.NewMigrator(db.Pool)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	log.Println("[Seed] Executing schema up migrations...")
	if err := migrator.RunMigrationFile(ctx, "./migrations/000001_init_schema.up.sql"); err != nil {
		log.Printf("[Seed] Note: %v (tables may already exist)", err)
	}

	log.Println("[Seed] Executing seed data script...")
	if err := migrator.RunMigrationFile(ctx, "./migrations/seed.sql"); err != nil {
		log.Fatalf("[Seed] Failed to apply seed data: %v", err)
	}

	log.Println("[Seed] Seed data successfully loaded!")
	log.Println("---------------------------------------------------------------------------------")
	log.Println("Seeded Tenants:")
	log.Println("  1. Acme Corporation  (code: acme-corp) -> Admin: bob@acme.com")
	log.Println("  2. Globex Corporation(code: globex)    -> Auditor: charlie@globex.com")
	log.Println("Multi-Tenant User:")
	log.Println("  - alice@multitenant.com (Admin in Acme, Auditor in Globex)")
	log.Println("Default Password for all seeded users: Password123!")
	log.Println("---------------------------------------------------------------------------------")
}
