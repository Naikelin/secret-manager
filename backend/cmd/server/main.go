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

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/yourorg/secret-manager/internal/api"
	"github.com/yourorg/secret-manager/internal/config"
	"github.com/yourorg/secret-manager/internal/database"
	"github.com/yourorg/secret-manager/internal/drift"
	"github.com/yourorg/secret-manager/internal/git"
	"github.com/yourorg/secret-manager/internal/k8s"
	"github.com/yourorg/secret-manager/internal/models"
	"github.com/yourorg/secret-manager/internal/notifications"
	"github.com/yourorg/secret-manager/internal/sops"
	"github.com/yourorg/secret-manager/pkg/logger"
	"gorm.io/gorm"
	"strconv"
	"strings"

	_ "github.com/yourorg/secret-manager/docs" // Import generated docs
)

// @title Secret Manager API
// @version 1.0
// @description Production-ready secret management with drift detection and GitOps integration
// @termsOfService https://github.com/yourorg/secret-manager

// @contact.name API Support
// @contact.url https://github.com/yourorg/secret-manager/issues
// @contact.email support@example.com

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @host localhost:8080
// @BasePath /api/v1
// @schemes http https

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

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

	// Initialize drift detector for background job
	var driftDetector *drift.DriftDetector
	k8sClient, gitClient, sopsClient := initClientsForDrift(cfg)
	if k8sClient != nil && gitClient != nil && sopsClient != nil {
		// Initialize webhook client
		webhookURL := os.Getenv("DRIFT_WEBHOOK_URL")
		webhookClient := notifications.NewWebhookClient(webhookURL)
		if webhookClient != nil {
			logger.Info("Webhook notifications enabled for drift detection", "url", webhookURL)
		}

		driftDetector = drift.NewDriftDetector(db, k8sClient, gitClient, sopsClient, webhookClient)
		logger.Info("Drift detector initialized successfully")
	} else {
		logger.Warn("Drift detector not initialized - some clients are unavailable")
	}

	// Start background drift detection job
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if driftDetector != nil {
		driftInterval := getEnvDuration("DRIFT_CHECK_INTERVAL", 5*time.Minute)
		go startDriftDetectionJob(ctx, db, driftDetector, driftInterval)
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

	// Cancel background jobs
	cancel()

	// Graceful shutdown with 10 second timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Fatal("Server forced to shutdown", "error", err)
	}

	logger.Info("Server exited")
}

// initClientsForDrift initializes K8s, Git, and SOPS clients for drift detection
func initClientsForDrift(cfg *config.Config) (*k8s.K8sClient, *git.GitClient, *sops.SOPSClient) {
	// Initialize K8s client
	k8sClient, err := k8s.NewK8sClient(cfg.K8sKubeconfig)
	if err != nil {
		logger.Warn("Failed to initialize K8s client for drift detection", "error", err)
		k8sClient = nil
	}

	// Initialize Git client
	var gitClient *git.GitClient
	if cfg.GitRepoURL != "" {
		var auth transport.AuthMethod
		if cfg.GitAuthType == "ssh" {
			auth, err = git.NewSSHAuth(cfg.GitSSHKeyPath)
			if err != nil {
				logger.Warn("Failed to create Git SSH auth", "error", err)
			}
		} else if cfg.GitAuthType == "token" {
			auth = git.NewTokenAuth(cfg.GitToken, "git")
		}

		gitClient, err = git.NewGitClient(cfg.GitRepoPath, cfg.GitRepoURL, cfg.GitBranch, auth)
		if err != nil {
			logger.Warn("Failed to initialize Git client for drift detection", "error", err)
			gitClient = nil
		} else {
			gitClient.SetAuthor(cfg.GitAuthorName, cfg.GitAuthorEmail)
		}
	}

	// Initialize SOPS client
	var sopsClient *sops.SOPSClient
	if cfg.SOPSEnabled {
		sopsClient, err = sops.NewSOPSClient(cfg.SOPSEncryptType, cfg.SOPSAgeKeyPath, cfg.SOPSKMSKeyARN, cfg.SOPSConfigPath)
		if err != nil {
			logger.Warn("Failed to initialize SOPS client for drift detection", "error", err)
			sopsClient = nil
		}
	}

	return k8sClient, gitClient, sopsClient
}

// startDriftDetectionJob runs drift detection periodically for all namespaces
func startDriftDetectionJob(ctx context.Context, db *gorm.DB, detector *drift.DriftDetector, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	logger.Info("Starting background drift detection job", "interval", interval)

	for {
		select {
		case <-ticker.C:
			checkDriftForAllNamespaces(db, detector)
		case <-ctx.Done():
			logger.Info("Stopping drift detection job")
			return
		}
	}
}

// checkDriftForAllNamespaces checks all namespaces for drift
func checkDriftForAllNamespaces(db *gorm.DB, detector *drift.DriftDetector) {
	var namespaces []models.Namespace
	if err := db.Find(&namespaces).Error; err != nil {
		logger.Error("Failed to fetch namespaces for drift check", "error", err)
		return
	}

	for _, ns := range namespaces {
		logger.Info("Running drift check", "namespace", ns.Name)

		events, err := detector.DetectDriftForNamespace(ns.ID)
		if err != nil {
			logger.Error("Drift detection failed", "namespace", ns.Name, "error", err)
			continue
		}

		if len(events) > 0 {
			logger.Warn("Drift detected", "namespace", ns.Name, "count", len(events))
		}
	}
}

// getEnvDuration parses a duration from environment variable with a default fallback
func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	// Try parsing as duration string (e.g., "5m", "1h")
	duration, err := time.ParseDuration(value)
	if err == nil {
		return duration
	}

	// Try parsing as integer seconds
	seconds, err := strconv.Atoi(strings.TrimSpace(value))
	if err == nil {
		return time.Duration(seconds) * time.Second
	}

	logger.Warn("Invalid duration in environment variable, using default", "key", key, "value", value, "default", defaultValue)
	return defaultValue
}
