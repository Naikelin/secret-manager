package database

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/yourorg/secret-manager/internal/models"
	"gorm.io/gorm"
)

// SeedData seeds the database with initial data (idempotent)
func SeedData(db *gorm.DB) error {
	// Check if data already exists
	var userCount int64
	db.Model(&models.User{}).Count(&userCount)
	if userCount > 0 {
		slog.Info("Database already seeded, skipping")
		return nil
	}

	slog.Info("Seeding database with initial data...")

	// Create users
	adminUser := models.User{
		Email: "admin@example.com",
		Name:  "Admin User",
	}
	if err := db.Create(&adminUser).Error; err != nil {
		return err
	}

	devUser := models.User{
		Email: "dev@example.com",
		Name:  "Developer User",
	}
	if err := db.Create(&devUser).Error; err != nil {
		return err
	}

	// Create groups
	adminsGroup := models.Group{
		Name: "admins",
	}
	if err := db.Create(&adminsGroup).Error; err != nil {
		return err
	}

	developersGroup := models.Group{
		Name: "developers",
	}
	if err := db.Create(&developersGroup).Error; err != nil {
		return err
	}

	// Associate users with groups
	db.Model(&adminUser).Association("Groups").Append(&adminsGroup)
	db.Model(&devUser).Association("Groups").Append(&developersGroup)

	// Create namespaces
	devNamespace := models.Namespace{
		Name:        "development",
		Cluster:     "dev-cluster",
		Environment: "dev",
	}
	if err := db.Create(&devNamespace).Error; err != nil {
		return err
	}

	stagingNamespace := models.Namespace{
		Name:        "staging",
		Cluster:     "staging-cluster",
		Environment: "staging",
	}
	if err := db.Create(&stagingNamespace).Error; err != nil {
		return err
	}

	prodNamespace := models.Namespace{
		Name:        "production",
		Cluster:     "prod-cluster",
		Environment: "prod",
	}
	if err := db.Create(&prodNamespace).Error; err != nil {
		return err
	}

	// Create group permissions
	permissions := []models.GroupPermission{
		{
			GroupID:     adminsGroup.ID,
			NamespaceID: devNamespace.ID,
			Role:        "admin",
		},
		{
			GroupID:     adminsGroup.ID,
			NamespaceID: stagingNamespace.ID,
			Role:        "admin",
		},
		{
			GroupID:     adminsGroup.ID,
			NamespaceID: prodNamespace.ID,
			Role:        "admin",
		},
		{
			GroupID:     developersGroup.ID,
			NamespaceID: devNamespace.ID,
			Role:        "editor",
		},
	}
	if err := db.Create(&permissions).Error; err != nil {
		return err
	}

	// Create sample draft
	draftData := map[string]string{
		"DB_HOST":     "postgres.dev.svc.cluster.local",
		"DB_PORT":     "5432",
		"DB_NAME":     "myapp",
		"DB_USER":     "appuser",
		"DB_PASSWORD": "change-me-in-production",
	}
	draftJSON, _ := json.Marshal(draftData)

	draft := models.SecretDraft{
		SecretName:  "db-credentials",
		NamespaceID: devNamespace.ID,
		Data:        draftJSON,
		Status:      "draft",
		EditedBy:    &devUser.ID,
		EditedAt:    time.Now(),
	}
	if err := db.Create(&draft).Error; err != nil {
		return err
	}

	slog.Info("Database seeded successfully",
		"users", 2,
		"groups", 2,
		"namespaces", 3,
		"permissions", 4,
		"drafts", 1,
	)

	return nil
}
