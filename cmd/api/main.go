package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/deb151292/neurealm-go-Assignment-/internal/config"
	"github.com/deb151292/neurealm-go-Assignment-/internal/database"
	"github.com/deb151292/neurealm-go-Assignment-/internal/server"
)

func main() {
	log.Println("[API] Starting Multi-Tenant User Management System...")

	// 1. Load Configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[API] Failed to load configuration: %v", err)
	}

	// 2. Initialize PostgreSQL connection pool using pgxpool
	db, err := database.NewPool(cfg)
	if err != nil {
		log.Fatalf("[API] Database connection error: %v", err)
	}
	defer db.Close()

	// 3. Auto-apply migrations using Goose
	migrator := database.NewMigrator(db.Pool, "./database/migrations")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	if err := migrator.Up(ctx); err != nil {
		log.Printf("[API] Migration warning: %v", err)
	}
	cancel()

	// 4. Initialize HTTP Server
	srv := server.NewServer(cfg, db.Pool)

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      srv.Router(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 5. Start Server in goroutine
	go func() {
		log.Printf("[API] Server listening on http://0.0.0.0:%s (Base: %s)", cfg.Port, cfg.APIBasePath)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("[API] Listen error: %v", err)
		}
	}()

	// 6. Graceful Shutdown on Interrupt
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[API] Shutting down server gracefully...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	srv.Cache().Stop()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("[API] Server forced to shutdown: %v", err)
	}

	log.Println("[API] Server exited cleanly.")
}
