package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// Config holds application configuration
type Config struct {
	Port           int
	DatabaseURL    string
	GitRepoPath    string
	K8sKubeconfig  string
	AuthProvider   string
	SOPSAgeKeyFile string
	LogLevel       string
	JWTSecret      string

	// Git configuration
	GitRepoURL     string
	GitBranch      string
	GitAuthType    string
	GitSSHKeyPath  string
	GitToken       string
	GitAuthorName  string
	GitAuthorEmail string

	// SOPS configuration
	SOPSEnabled     bool
	SOPSEncryptType string
	SOPSAgeKeyPath  string
	SOPSKMSKeyARN   string
	SOPSConfigPath  string
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	// Load .env file if it exists (optional - doesn't error if missing)
	_ = godotenv.Load()

	port := getEnvInt("PORT", 8080)

	cfg := &Config{
		Port:           port,
		DatabaseURL:    getEnv("DATABASE_URL", "postgres://dev:devpass@localhost:5432/secretmanager?sslmode=disable"),
		GitRepoPath:    getEnv("GIT_REPO_PATH", "/data/secrets-repo"),
		K8sKubeconfig:  getEnv("K8S_KUBECONFIG", ""),
		AuthProvider:   getEnv("AUTH_PROVIDER", "mock"),
		SOPSAgeKeyFile: getEnv("SOPS_AGE_KEY_FILE", "/keys/age.txt"),
		LogLevel:       getEnv("LOG_LEVEL", "info"),
		JWTSecret:      getEnv("JWT_SECRET", "dev-secret-change-in-production"),

		// Git configuration
		GitRepoURL:     getEnv("GIT_REPO_URL", ""),
		GitBranch:      getEnv("GIT_BRANCH", "main"),
		GitAuthType:    getEnv("GIT_AUTH_TYPE", "ssh"),
		GitSSHKeyPath:  getEnv("GIT_SSH_KEY_PATH", ""),
		GitToken:       getEnv("GIT_TOKEN", ""),
		GitAuthorName:  getEnv("GIT_AUTHOR_NAME", "Secret Manager"),
		GitAuthorEmail: getEnv("GIT_AUTHOR_EMAIL", "secret-manager@example.com"),

		// SOPS configuration
		SOPSEnabled:     getEnvBool("SOPS_ENABLED", true),
		SOPSEncryptType: getEnv("SOPS_ENCRYPT_TYPE", "age"),
		SOPSAgeKeyPath:  getEnv("SOPS_AGE_KEY_PATH", "/app/.age/keys.txt"),
		SOPSKMSKeyARN:   getEnv("SOPS_KMS_KEY_ARN", ""),
		SOPSConfigPath:  getEnv("SOPS_CONFIG_PATH", ".sops.yaml"),
	}

	return cfg, nil
}

// InitDatabase initializes database connection with GORM
func InitDatabase(cfg *Config) (*gorm.DB, error) {
	// Configure GORM logger based on log level
	var logLevel gormlogger.LogLevel
	switch cfg.LogLevel {
	case "debug":
		logLevel = gormlogger.Info
	case "error":
		logLevel = gormlogger.Error
	default:
		logLevel = gormlogger.Warn
	}

	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{
		Logger: gormlogger.Default.LogMode(logLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Get underlying SQL DB for connection pool configuration
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying database: %w", err)
	}

	// Configure connection pool
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)

	return db, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}
