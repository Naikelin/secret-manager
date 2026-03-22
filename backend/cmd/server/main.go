package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yourorg/secret-manager/internal/api"
	"github.com/yourorg/secret-manager/internal/config"
	"github.com/yourorg/secret-manager/internal/database"
	"github.com/yourorg/secret-manager/internal/models"
	"github.com/yourorg/secret-manager/pkg/logger"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger
	logger.Init(cfg.LogLevel)
	logger.Info("Starting secret-manager backend", "version", "0.1.0", "port", cfg.Port)

	// Initialize database
	db, err := config.InitDatabase(cfg)
	if err != nil {
		logger.Fatal("Failed to initialize database", "error", err)
	}

	logger.Info("Database connection established")

	// Run migrations
	logger.Info("Running database migrations...")
	err = db.AutoMigrate(
		&models.User{},
		&models.Group{},
		&models.Namespace{},
		&models.GroupPermission{},
		&models.SecretDraft{},
		&models.DriftEvent{},
		&models.AuditLog{},
	)
	if err != nil {
		logger.Fatal("Failed to migrate database", "error", err)
	}

	// Seed initial data
	if err := database.SeedData(db); err != nil {
		logger.Fatal("Failed to seed database", "error", err)
	}

	// Create router
	router := api.NewRouter(db, cfg)

	// Create HTTP server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		logger.Info("Server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server failed to start", "error", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Graceful shutdown with 10 second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", "error", err)
	}

	logger.Info("Server exited")
}
