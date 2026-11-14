package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/deb151292/neurealm-go-Assignment-/internal/config"
	"github.com/deb151292/neurealm-go-Assignment-/internal/database"
)

func main() {
	dir := flag.String("dir", "./database/migrations", "directory containing goose migration files")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		fmt.Println("Usage: go run cmd/migrate/main.go [-dir ./database/migrations] <up|down|status>")
		os.Exit(1)
	}

	command := args[0]

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[Migrate] Failed to load config: %v", err)
	}

	db, err := database.NewPool(cfg)
	if err != nil {
		log.Fatalf("[Migrate] Failed to connect to database: %v", err)
	}
	defer db.Close()

	migrator := database.NewMigrator(db.Pool, *dir)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	switch command {
	case "up":
		if err := migrator.Up(ctx); err != nil {
			log.Fatalf("[Migrate] Migration UP failed: %v", err)
		}
	case "down":
		if err := migrator.Down(ctx); err != nil {
			log.Fatalf("[Migrate] Migration DOWN failed: %v", err)
		}
	case "status":
		if err := migrator.Status(ctx); err != nil {
			log.Fatalf("[Migrate] Migration STATUS failed: %v", err)
		}
	default:
		log.Fatalf("[Migrate] Unknown command '%s'. Valid commands: up, down, status", command)
	}
}
