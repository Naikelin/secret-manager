package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/yourorg/secret-manager/internal/models"
	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	// Get database connection from environment
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	// Connect to database
	db, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	fmt.Println("🌱 Starting E2E seed data creation...")
	fmt.Println("")

	// Step 1: Run migrations to ensure all tables exist
	fmt.Println("📋 Running database migrations...")
	if err := runMigrations(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	fmt.Println("✓ Migrations complete")
	fmt.Println("")

	// Step 2: Clean existing test data
	fmt.Println("🧹 Cleaning existing test data...")
	if err := cleanTestData(db); err != nil {
		log.Fatalf("Failed to clean test data: %v", err)
	}
	fmt.Println("✓ Cleaned existing test data")
	fmt.Println("")

	// Step 3: Create test users
	fmt.Println("👤 Creating test users...")
	users, err := createUsers(db)
	if err != nil {
		log.Fatalf("Failed to create users: %v", err)
	}
	fmt.Printf("✓ Created %d users\n", len(users))
	for _, u := range users {
		fmt.Printf("  - %s (%s)\n", u.Email, u.ID)
	}
	fmt.Println("")

	// Step 4: Create namespaces
	fmt.Println("📦 Creating namespaces...")
	namespaces, err := createNamespaces(db)
	if err != nil {
		log.Fatalf("Failed to create namespaces: %v", err)
	}
	fmt.Printf("✓ Created %d namespaces\n", len(namespaces))
	for _, ns := range namespaces {
		fmt.Printf("  - %s (%s) [%s]\n", ns.Name, ns.Environment, ns.ID)
	}
	fmt.Println("")

	// Step 5: Create secrets
	fmt.Println("🔐 Creating secrets...")
	secrets, err := createSecrets(db, namespaces, users)
	if err != nil {
		log.Fatalf("Failed to create secrets: %v", err)
	}
	fmt.Printf("✓ Created %d secrets\n", len(secrets))
	for _, s := range secrets {
		fmt.Printf("  - %s [%s] in namespace %s\n", s.SecretName, s.Status, s.NamespaceID)
	}
	fmt.Println("")

	// Step 6: Create drift events
	fmt.Println("⚠️  Creating drift events...")
	driftEvents, err := createDriftEvents(db, namespaces, users)
	if err != nil {
		log.Fatalf("Failed to create drift events: %v", err)
	}
	fmt.Printf("✓ Created %d drift events\n", len(driftEvents))
	for _, d := range driftEvents {
		status := "detected"
		if d.ResolvedAt != nil {
			status = "resolved"
		}
		fmt.Printf("  - %s [%s] in namespace %s\n", d.SecretName, status, d.NamespaceID)
	}
	fmt.Println("")

	fmt.Println("✅ E2E seed data creation complete!")
	fmt.Println("")
	fmt.Println("Summary:")
	fmt.Printf("  Users:        %d\n", len(users))
	fmt.Printf("  Namespaces:   %d\n", len(namespaces))
	fmt.Printf("  Secrets:      %d\n", len(secrets))
	fmt.Printf("  Drift Events: %d\n", len(driftEvents))
}

func runMigrations(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.User{},
		&models.Group{},
		&models.Namespace{},
		&models.GroupPermission{},
		&models.SecretDraft{},
		&models.DriftEvent{},
		&models.AuditLog{},
	)
}

func cleanTestData(db *gorm.DB) error {
	// Delete in reverse order of dependencies
	tables := []interface{}{
		&models.AuditLog{},
		&models.DriftEvent{},
		&models.SecretDraft{},
		&models.GroupPermission{},
		&models.Namespace{},
		&models.Group{},
		&models.User{},
	}

	for _, table := range tables {
		if err := db.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(table).Error; err != nil {
			return fmt.Errorf("failed to clean %T: %w", table, err)
		}
	}

	return nil
}

func createUsers(db *gorm.DB) ([]models.User, error) {
	users := []models.User{
		{
			ID:    uuid.MustParse("00000000-0000-0000-0000-000000000001"),
			Email: "admin@example.com",
			Name:  "Admin User",
		},
		{
			ID:    uuid.MustParse("00000000-0000-0000-0000-000000000002"),
			Email: "test@example.com",
			Name:  "Test User",
		},
		{
			ID:    uuid.MustParse("00000000-0000-0000-0000-000000000003"),
			Email: "developer@example.com",
			Name:  "Developer User",
		},
	}

	for i := range users {
		if err := db.Create(&users[i]).Error; err != nil {
			return nil, err
		}
	}

	return users, nil
}

func createNamespaces(db *gorm.DB) ([]models.Namespace, error) {
	namespaces := []models.Namespace{
		{
			ID:          uuid.MustParse("10000000-0000-0000-0000-000000000001"),
			Name:        "development",
			Cluster:     "dev-cluster-1",
			Environment: "dev",
		},
		{
			ID:          uuid.MustParse("10000000-0000-0000-0000-000000000002"),
			Name:        "staging",
			Cluster:     "staging-cluster-1",
			Environment: "staging",
		},
		{
			ID:          uuid.MustParse("10000000-0000-0000-0000-000000000003"),
			Name:        "production",
			Cluster:     "prod-cluster-1",
			Environment: "prod",
		},
	}

	for i := range namespaces {
		if err := db.Create(&namespaces[i]).Error; err != nil {
			return nil, err
		}
	}

	return namespaces, nil
}

func createSecrets(db *gorm.DB, namespaces []models.Namespace, users []models.User) ([]models.SecretDraft, error) {
	now := time.Now()
	publishedAt := now.Add(-24 * time.Hour)
	adminID := users[0].ID

	secrets := []models.SecretDraft{
		// Development secrets
		{
			ID:          uuid.New(),
			SecretName:  "api-credentials",
			NamespaceID: namespaces[0].ID, // development
			Data:        mustJSON(map[string]string{"api_key": "dev_12345", "api_secret": "secret_abc"}),
			Status:      "published",
			GitBaseSHA:  "abc123def456",
			EditedBy:    &adminID,
			EditedAt:    publishedAt,
			PublishedBy: &adminID,
			PublishedAt: &publishedAt,
			CommitSHA:   "abc123def456",
		},
		{
			ID:          uuid.New(),
			SecretName:  "database-config",
			NamespaceID: namespaces[0].ID, // development
			Data:        mustJSON(map[string]string{"db_host": "localhost", "db_port": "5432", "db_password": "devpass"}),
			Status:      "published",
			GitBaseSHA:  "def456ghi789",
			EditedBy:    &adminID,
			EditedAt:    publishedAt,
			PublishedBy: &adminID,
			PublishedAt: &publishedAt,
			CommitSHA:   "def456ghi789",
		},
		{
			ID:          uuid.New(),
			SecretName:  "test-secret",
			NamespaceID: namespaces[0].ID, // development
			Data:        mustJSON(map[string]string{"username": "testuser", "password": "testpass"}),
			Status:      "draft",
			EditedBy:    &adminID,
			EditedAt:    now,
		},

		// Staging secrets
		{
			ID:          uuid.New(),
			SecretName:  "api-credentials",
			NamespaceID: namespaces[1].ID, // staging
			Data:        mustJSON(map[string]string{"api_key": "staging_67890", "api_secret": "secret_xyz"}),
			Status:      "published",
			GitBaseSHA:  "ghi789jkl012",
			EditedBy:    &adminID,
			EditedAt:    publishedAt,
			PublishedBy: &adminID,
			PublishedAt: &publishedAt,
			CommitSHA:   "ghi789jkl012",
		},
		{
			ID:          uuid.New(),
			SecretName:  "oauth-config",
			NamespaceID: namespaces[1].ID, // staging
			Data:        mustJSON(map[string]string{"client_id": "staging-client", "client_secret": "staging-secret"}),
			Status:      "published",
			GitBaseSHA:  "jkl012mno345",
			EditedBy:    &adminID,
			EditedAt:    publishedAt,
			PublishedBy: &adminID,
			PublishedAt: &publishedAt,
			CommitSHA:   "jkl012mno345",
		},

		// Production secrets
		{
			ID:          uuid.New(),
			SecretName:  "api-credentials",
			NamespaceID: namespaces[2].ID, // production
			Data:        mustJSON(map[string]string{"api_key": "prod_secure_key", "api_secret": "prod_secure_secret"}),
			Status:      "published",
			GitBaseSHA:  "mno345pqr678",
			EditedBy:    &adminID,
			EditedAt:    publishedAt,
			PublishedBy: &adminID,
			PublishedAt: &publishedAt,
			CommitSHA:   "mno345pqr678",
		},
		{
			ID:          uuid.New(),
			SecretName:  "database-config",
			NamespaceID: namespaces[2].ID, // production
			Data:        mustJSON(map[string]string{"db_host": "prod-db.example.com", "db_port": "5432", "db_password": "prod_secure_pass"}),
			Status:      "published",
			GitBaseSHA:  "pqr678stu901",
			EditedBy:    &adminID,
			EditedAt:    publishedAt,
			PublishedBy: &adminID,
			PublishedAt: &publishedAt,
			CommitSHA:   "pqr678stu901",
		},
		{
			ID:          uuid.New(),
			SecretName:  "tls-certificates",
			NamespaceID: namespaces[2].ID, // production
			Data:        mustJSON(map[string]string{"tls.crt": "-----BEGIN CERTIFICATE-----", "tls.key": "-----BEGIN PRIVATE KEY-----"}),
			Status:      "published",
			GitBaseSHA:  "stu901vwx234",
			EditedBy:    &adminID,
			EditedAt:    publishedAt,
			PublishedBy: &adminID,
			PublishedAt: &publishedAt,
			CommitSHA:   "stu901vwx234",
		},
		{
			ID:          uuid.New(),
			SecretName:  "smtp-config",
			NamespaceID: namespaces[2].ID, // production
			Data:        mustJSON(map[string]string{"smtp_host": "smtp.example.com", "smtp_user": "mailer", "smtp_password": "mail_pass"}),
			Status:      "draft",
			EditedBy:    &adminID,
			EditedAt:    now,
		},
	}

	for i := range secrets {
		if err := db.Create(&secrets[i]).Error; err != nil {
			return nil, err
		}
	}

	return secrets, nil
}

func createDriftEvents(db *gorm.DB, namespaces []models.Namespace, users []models.User) ([]models.DriftEvent, error) {
	now := time.Now()
	resolvedAt := now.Add(-2 * time.Hour)
	adminID := users[0].ID

	driftEvents := []models.DriftEvent{
		// Unresolved drift - modified values
		{
			ID:          uuid.New(),
			SecretName:  "api-credentials",
			NamespaceID: namespaces[0].ID, // development
			DetectedAt:  now.Add(-30 * time.Minute),
			GitVersion: mustJSON(map[string]interface{}{
				"data": map[string]string{
					"api_key":    "dev_12345",
					"api_secret": "secret_abc",
				},
			}),
			K8sVersion: mustJSON(map[string]interface{}{
				"data": map[string]string{
					"api_key":    "dev_12345_MODIFIED",
					"api_secret": "secret_abc",
				},
			}),
			Diff: mustJSON(map[string]interface{}{
				"keys_added":    []string{},
				"keys_removed":  []string{},
				"keys_modified": []string{"api_key"},
				"summary":       "1 key modified",
			}),
		},

		// Unresolved drift - key added in K8s
		{
			ID:          uuid.New(),
			SecretName:  "database-config",
			NamespaceID: namespaces[0].ID, // development
			DetectedAt:  now.Add(-15 * time.Minute),
			GitVersion: mustJSON(map[string]interface{}{
				"data": map[string]string{
					"db_host":     "localhost",
					"db_port":     "5432",
					"db_password": "devpass",
				},
			}),
			K8sVersion: mustJSON(map[string]interface{}{
				"data": map[string]string{
					"db_host":     "localhost",
					"db_port":     "5432",
					"db_password": "devpass",
					"db_user":     "postgres", // Extra key in K8s
				},
			}),
			Diff: mustJSON(map[string]interface{}{
				"keys_added":    []string{"db_user"},
				"keys_removed":  []string{},
				"keys_modified": []string{},
				"summary":       "1 key added",
			}),
		},

		// Unresolved drift - key removed from K8s
		{
			ID:          uuid.New(),
			SecretName:  "oauth-config",
			NamespaceID: namespaces[1].ID, // staging
			DetectedAt:  now.Add(-45 * time.Minute),
			GitVersion: mustJSON(map[string]interface{}{
				"data": map[string]string{
					"client_id":     "staging-client",
					"client_secret": "staging-secret",
					"redirect_uri":  "https://staging.example.com/callback",
				},
			}),
			K8sVersion: mustJSON(map[string]interface{}{
				"data": map[string]string{
					"client_id":     "staging-client",
					"client_secret": "staging-secret",
					// redirect_uri missing in K8s
				},
			}),
			Diff: mustJSON(map[string]interface{}{
				"keys_added":    []string{},
				"keys_removed":  []string{"redirect_uri"},
				"keys_modified": []string{},
				"summary":       "1 key removed",
			}),
		},

		// Resolved drift - synced from git
		{
			ID:          uuid.New(),
			SecretName:  "database-config",
			NamespaceID: namespaces[2].ID, // production
			DetectedAt:  now.Add(-3 * time.Hour),
			GitVersion: mustJSON(map[string]interface{}{
				"data": map[string]string{
					"db_host":     "prod-db.example.com",
					"db_port":     "5432",
					"db_password": "prod_secure_pass",
				},
			}),
			K8sVersion: mustJSON(map[string]interface{}{
				"data": map[string]string{
					"db_host":     "old-db.example.com",
					"db_port":     "5432",
					"db_password": "old_password",
				},
			}),
			Diff: mustJSON(map[string]interface{}{
				"keys_added":    []string{},
				"keys_removed":  []string{},
				"keys_modified": []string{"db_host", "db_password"},
				"summary":       "2 keys modified",
			}),
			ResolvedAt:       &resolvedAt,
			ResolvedBy:       &adminID,
			ResolutionAction: stringPtr("sync_from_git"),
		},

		// Resolved drift - marked as ignored
		{
			ID:          uuid.New(),
			SecretName:  "tls-certificates",
			NamespaceID: namespaces[2].ID, // production
			DetectedAt:  now.Add(-5 * time.Hour),
			GitVersion: mustJSON(map[string]interface{}{
				"data": map[string]string{
					"tls.crt": "-----BEGIN CERTIFICATE-----",
					"tls.key": "-----BEGIN PRIVATE KEY-----",
				},
			}),
			K8sVersion: mustJSON(map[string]interface{}{
				"data": map[string]string{
					"tls.crt": "-----BEGIN CERTIFICATE----- (renewed)",
					"tls.key": "-----BEGIN PRIVATE KEY-----",
				},
			}),
			Diff: mustJSON(map[string]interface{}{
				"keys_added":    []string{},
				"keys_removed":  []string{},
				"keys_modified": []string{"tls.crt"},
				"summary":       "1 key modified (certificate renewed)",
			}),
			ResolvedAt:       &resolvedAt,
			ResolvedBy:       &adminID,
			ResolutionAction: stringPtr("ignore"),
		},
	}

	for i := range driftEvents {
		if err := db.Create(&driftEvents[i]).Error; err != nil {
			return nil, err
		}
	}

	return driftEvents, nil
}

func mustJSON(v interface{}) datatypes.JSON {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal JSON: %v", err))
	}
	return datatypes.JSON(b)
}

func stringPtr(s string) *string {
	return &s
}
