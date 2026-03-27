package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/yourorg/secret-manager/internal/config"
	"github.com/yourorg/secret-manager/internal/models"
	"gorm.io/gorm"
)

// BackfillClusterData migrates data from namespaces.cluster (TEXT) to clusters table + cluster_id FK
func main() {
	fmt.Println("=== Cluster Data Backfill Script ===")
	fmt.Println("This script will:")
	fmt.Println("1. Extract distinct cluster names from namespaces.cluster")
	fmt.Println("2. Insert them into the clusters table")
	fmt.Println("3. Update namespaces.cluster_id to reference the new clusters")
	fmt.Println()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize database
	db, err := config.InitDatabase(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("Failed to get SQL DB: %v", err)
	}
	defer sqlDB.Close()

	ctx := context.Background()

	// Start transaction
	tx := db.WithContext(ctx).Begin()
	if tx.Error != nil {
		log.Fatalf("Failed to start transaction: %v", tx.Error)
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			log.Fatalf("Panic during backfill: %v", r)
		}
	}()

	// Step 1: Get distinct cluster names from namespaces
	type ClusterNameEnv struct {
		Cluster     string
		Environment string
	}

	var distinctClusters []ClusterNameEnv
	if err := tx.Model(&models.Namespace{}).
		Select("DISTINCT cluster, environment").
		Where("cluster != ''").
		Find(&distinctClusters).Error; err != nil {
		tx.Rollback()
		log.Fatalf("Failed to query distinct clusters: %v", err)
	}

	fmt.Printf("Found %d distinct cluster(s) in namespaces table\n", len(distinctClusters))

	if len(distinctClusters) == 0 {
		fmt.Println("No clusters found to migrate. Exiting.")
		tx.Rollback()
		return
	}

	// Step 2: Insert clusters into clusters table
	insertedCount := 0
	skippedCount := 0
	clusterMap := make(map[string]uuid.UUID) // cluster name -> cluster ID

	for _, cn := range distinctClusters {
		// Check if cluster already exists
		var existingCluster models.Cluster
		err := tx.Where("name = ?", cn.Cluster).First(&existingCluster).Error

		if err == nil {
			// Cluster exists
			clusterMap[cn.Cluster] = existingCluster.ID
			skippedCount++
			fmt.Printf("  [SKIP] Cluster '%s' already exists (ID: %s)\n", cn.Cluster, existingCluster.ID)
			continue
		} else if err != gorm.ErrRecordNotFound {
			tx.Rollback()
			log.Fatalf("Error checking cluster existence: %v", err)
		}

		// Insert new cluster
		newCluster := models.Cluster{
			Name:          cn.Cluster,
			KubeconfigRef: fmt.Sprintf("/etc/kubeconfigs/%s.yaml", cn.Cluster),
			Environment:   cn.Environment,
			IsHealthy:     true,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}

		if err := tx.Create(&newCluster).Error; err != nil {
			tx.Rollback()
			log.Fatalf("Failed to insert cluster '%s': %v", cn.Cluster, err)
		}

		clusterMap[cn.Cluster] = newCluster.ID
		insertedCount++
		fmt.Printf("  [INSERT] Created cluster '%s' (ID: %s, Env: %s)\n",
			newCluster.Name, newCluster.ID, newCluster.Environment)
	}

	fmt.Printf("\nCluster table update: %d inserted, %d skipped\n\n", insertedCount, skippedCount)

	// Step 3: Update namespaces.cluster_id
	var namespaces []models.Namespace
	if err := tx.Find(&namespaces).Error; err != nil {
		tx.Rollback()
		log.Fatalf("Failed to load namespaces: %v", err)
	}

	fmt.Printf("Updating %d namespace(s) with cluster_id...\n", len(namespaces))

	updatedCount := 0
	for _, ns := range namespaces {
		clusterID, exists := clusterMap[ns.Cluster]
		if !exists {
			tx.Rollback()
			log.Fatalf("FATAL: Namespace '%s' (ID: %s) references unknown cluster '%s'",
				ns.Name, ns.ID, ns.Cluster)
		}

		// Update cluster_id
		if err := tx.Model(&ns).Update("cluster_id", clusterID).Error; err != nil {
			tx.Rollback()
			log.Fatalf("Failed to update namespace '%s' (ID: %s): %v", ns.Name, ns.ID, err)
		}

		updatedCount++
		if updatedCount%10 == 0 || updatedCount == len(namespaces) {
			fmt.Printf("  Updated %d/%d namespaces\n", updatedCount, len(namespaces))
		}
	}

	// Step 4: Verify no NULL cluster_id values
	var nullCount int64
	if err := tx.Model(&models.Namespace{}).Where("cluster_id IS NULL").Count(&nullCount).Error; err != nil {
		tx.Rollback()
		log.Fatalf("Failed to verify cluster_id: %v", err)
	}

	if nullCount > 0 {
		tx.Rollback()
		log.Fatalf("FATAL: %d namespaces still have NULL cluster_id after backfill!", nullCount)
	}

	fmt.Println("\n✅ Verification passed: All namespaces have cluster_id set")

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		log.Fatalf("Failed to commit transaction: %v", err)
	}

	fmt.Println("\n=== Backfill Complete ===")
	fmt.Printf("Summary:\n")
	fmt.Printf("  - Clusters created: %d\n", insertedCount)
	fmt.Printf("  - Clusters skipped (already exist): %d\n", skippedCount)
	fmt.Printf("  - Namespaces updated: %d\n", updatedCount)
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Verify data integrity: SELECT * FROM namespaces WHERE cluster_id IS NULL;")
	fmt.Println("  2. Run migration 010 to drop old cluster column and enforce NOT NULL constraint")
}
