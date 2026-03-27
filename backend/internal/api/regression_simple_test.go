package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourorg/secret-manager/internal/models"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// setupTestDBWithClusters extends setupTestDB to include clusters and user_groups tables
func setupTestDBWithClusters(t *testing.T) *gorm.DB {
	db := setupTestDB(t)

	// Add user_groups table
	err := db.Exec(`
		CREATE TABLE user_groups (
			user_id TEXT NOT NULL,
			group_id TEXT NOT NULL,
			PRIMARY KEY (user_id, group_id)
		)
	`).Error
	require.NoError(t, err)

	// Add clusters table
	err = db.Exec(`
		CREATE TABLE clusters (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			kubeconfig_ref TEXT NOT NULL,
			environment TEXT NOT NULL CHECK(environment IN ('development', 'staging', 'production')),
			is_healthy BOOLEAN DEFAULT true,
			last_health_check DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)
	`).Error
	require.NoError(t, err)

	// Update namespaces table to include cluster_id
	err = db.Exec(`DROP TABLE IF EXISTS namespaces`).Error
	require.NoError(t, err)

	err = db.Exec(`
		CREATE TABLE namespaces (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			cluster_id TEXT,
			cluster TEXT NOT NULL,
			environment TEXT NOT NULL CHECK(environment IN ('dev', 'staging', 'prod')),
			created_at DATETIME,
			FOREIGN KEY (cluster_id) REFERENCES clusters(id) ON DELETE CASCADE
		)
	`).Error
	require.NoError(t, err)

	return db
}

// TestBackwardCompat_NamespaceWithCluster verifies that namespaces can be created
// and retrieved with the legacy Cluster TEXT field still populated alongside ClusterID.
func TestBackwardCompat_NamespaceWithCluster(t *testing.T) {
	db := setupTestDBWithClusters(t)

	// Create a cluster
	cluster := models.Cluster{
		ID:            uuid.New(),
		Name:          "legacy-cluster",
		KubeconfigRef: "fake-kubeconfig",
		Environment:   "development",
	}
	require.NoError(t, db.Create(&cluster).Error)

	// Create namespace with BOTH cluster_id (new) AND cluster (legacy TEXT field)
	namespace := models.Namespace{
		ID:          uuid.New(),
		Name:        "legacy-ns",
		ClusterID:   &cluster.ID,      // New FK field
		Cluster:     "legacy-cluster", // Old TEXT field (deprecated but still present)
		Environment: "dev",
	}
	require.NoError(t, db.Create(&namespace).Error)

	// Retrieve the namespace from DB
	var retrieved models.Namespace
	err := db.First(&retrieved, "id = ?", namespace.ID).Error
	require.NoError(t, err)

	// Verify both fields are preserved
	assert.Equal(t, "legacy-cluster", retrieved.Cluster, "Legacy Cluster TEXT field should be preserved")
	assert.NotNil(t, retrieved.ClusterID, "ClusterID FK should be set")
	assert.Equal(t, cluster.ID, *retrieved.ClusterID, "ClusterID should reference the correct cluster")
}

// TestBackwardCompat_SecretListAPI verifies that the GET /namespaces/{ns-id}/secrets
// API response includes all expected fields and the response format is unchanged.
func TestBackwardCompat_SecretListAPI(t *testing.T) {
	db := setupTestDBWithClusters(t)

	// Create user
	user := models.User{
		ID:    uuid.New(),
		Email: "test@example.com",
		Name:  "Test User",
	}
	require.NoError(t, db.Create(&user).Error)

	// Create group
	group := models.Group{
		ID:   uuid.New(),
		Name: "test-group",
	}
	require.NoError(t, db.Create(&group).Error)

	// Associate user with group
	err := db.Exec("INSERT INTO user_groups (user_id, group_id) VALUES (?, ?)", user.ID, group.ID).Error
	require.NoError(t, err)

	// Create cluster
	cluster := models.Cluster{
		ID:            uuid.New(),
		Name:          "api-test-cluster",
		KubeconfigRef: "fake-config",
		Environment:   "development",
	}
	require.NoError(t, db.Create(&cluster).Error)

	// Create namespace with both cluster fields
	namespace := models.Namespace{
		ID:          uuid.New(),
		Name:        "api-test-ns",
		ClusterID:   &cluster.ID,
		Cluster:     "api-test-cluster",
		Environment: "dev",
	}
	require.NoError(t, db.Create(&namespace).Error)

	// Create permission
	permission := models.GroupPermission{
		ID:          uuid.New(),
		GroupID:     group.ID,
		NamespaceID: namespace.ID,
		Role:        "editor",
	}
	require.NoError(t, db.Create(&permission).Error)

	// Create a secret draft
	secret := models.SecretDraft{
		ID:          uuid.New(),
		SecretName:  "test-secret",
		NamespaceID: namespace.ID,
		Data:        datatypes.JSON(`{"key1":"value1"}`),
		Status:      "draft",
	}
	require.NoError(t, db.Create(&secret).Error)

	// Create handlers and make API request
	handlers := NewSecretHandlers(db, nil) // nil GitSync for this test

	req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/"+namespace.ID.String()+"/secrets", nil)
	req = req.WithContext(context.WithValue(req.Context(), "user_id", user.ID))

	// Add chi URL params
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("namespace", namespace.ID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	handlers.ListSecrets(rr, req)

	// Assert successful response
	assert.Equal(t, http.StatusOK, rr.Code)

	// Decode response
	var secrets []models.SecretDraft
	err = json.NewDecoder(rr.Body).Decode(&secrets)
	require.NoError(t, err)

	// Verify response structure includes expected fields
	require.Len(t, secrets, 1)
	assert.Equal(t, "test-secret", secrets[0].SecretName)
	assert.Equal(t, namespace.ID, secrets[0].NamespaceID)
	assert.Equal(t, "draft", secrets[0].Status)
	assert.NotNil(t, secrets[0].Data)
}

// TestBackwardCompat_DualClusterFields verifies that creating a namespace with
// both cluster_id (FK) and cluster (TEXT) preserves both values correctly.
// This is critical during migration when both fields coexist.
func TestBackwardCompat_DualClusterFields(t *testing.T) {
	db := setupTestDBWithClusters(t)

	// Create two clusters
	clusterA := models.Cluster{
		ID:            uuid.New(),
		Name:          "cluster-a",
		KubeconfigRef: "config-a",
		Environment:   "development",
	}
	require.NoError(t, db.Create(&clusterA).Error)

	clusterB := models.Cluster{
		ID:            uuid.New(),
		Name:          "cluster-b",
		KubeconfigRef: "config-b",
		Environment:   "staging",
	}
	require.NoError(t, db.Create(&clusterB).Error)

	// Create two namespaces, each with cluster_id + cluster fields
	nsA := models.Namespace{
		ID:          uuid.New(),
		Name:        "ns-a",
		ClusterID:   &clusterA.ID,
		Cluster:     "cluster-a",
		Environment: "dev",
	}
	require.NoError(t, db.Create(&nsA).Error)

	nsB := models.Namespace{
		ID:          uuid.New(),
		Name:        "ns-b",
		ClusterID:   &clusterB.ID,
		Cluster:     "cluster-b",
		Environment: "staging",
	}
	require.NoError(t, db.Create(&nsB).Error)

	// Retrieve both namespaces
	var retrievedA, retrievedB models.Namespace
	err := db.First(&retrievedA, "id = ?", nsA.ID).Error
	require.NoError(t, err)
	err = db.First(&retrievedB, "id = ?", nsB.ID).Error
	require.NoError(t, err)

	// Verify namespace A has correct dual fields
	assert.Equal(t, "cluster-a", retrievedA.Cluster)
	assert.Equal(t, clusterA.ID, *retrievedA.ClusterID)

	// Verify namespace B has correct dual fields
	assert.Equal(t, "cluster-b", retrievedB.Cluster)
	assert.Equal(t, clusterB.ID, *retrievedB.ClusterID)

	// Verify namespaces are distinct and not mixed up
	assert.NotEqual(t, retrievedA.ClusterID, retrievedB.ClusterID)
	assert.NotEqual(t, retrievedA.Cluster, retrievedB.Cluster)
}
