package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourorg/secret-manager/internal/k8s"
	"github.com/yourorg/secret-manager/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func setupK8sSecretsTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)

	// Manually create tables without PostgreSQL-specific features
	err = db.Exec(`
		CREATE TABLE namespaces (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			cluster TEXT NOT NULL,
			environment TEXT NOT NULL CHECK (environment IN ('dev', 'staging', 'prod')),
			created_at DATETIME
		)
	`).Error
	require.NoError(t, err)

	err = db.Exec(`
		CREATE TABLE secret_drafts (
			id TEXT PRIMARY KEY,
			namespace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			data TEXT,
			status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'drifted')),
			last_published_commit TEXT,
			published_by TEXT,
			published_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME,
			FOREIGN KEY (namespace_id) REFERENCES namespaces(id)
		)
	`).Error
	require.NoError(t, err)

	return db
}

func TestListK8sSecrets_K8sUnavailable(t *testing.T) {
	db := setupK8sSecretsTestDB(t)

	// Create test namespace
	namespace := models.Namespace{
		ID:          uuid.New(),
		Name:        "default",
		Cluster:     "test-cluster",
		Environment: "dev",
	}
	require.NoError(t, db.Create(&namespace).Error)

	handlers := NewK8sSecretHandlers(db, nil) // K8s client is nil

	req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/"+namespace.ID.String()+"/k8s-secrets", nil)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("namespace", namespace.ID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()

	handlers.ListK8sSecrets(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, rec.Body.String(), "Kubernetes cluster not available")
}

func TestListK8sSecrets_NamespaceNotFound(t *testing.T) {
	db := setupK8sSecretsTestDB(t)

	handlers := NewK8sSecretHandlers(db, &k8s.K8sClient{})

	// Use non-existent namespace ID
	nonExistentID := uuid.New()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/"+nonExistentID.String()+"/k8s-secrets", nil)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("namespace", nonExistentID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()

	handlers.ListK8sSecrets(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "Namespace not found")
}

func TestGetK8sSecret_K8sUnavailable(t *testing.T) {
	db := setupK8sSecretsTestDB(t)

	// Create test namespace
	namespace := models.Namespace{
		ID:          uuid.New(),
		Name:        "default",
		Cluster:     "test-cluster",
		Environment: "dev",
	}
	require.NoError(t, db.Create(&namespace).Error)

	handlers := NewK8sSecretHandlers(db, nil) // K8s client is nil

	req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/"+namespace.ID.String()+"/k8s-secrets/test-secret", nil)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("namespace", namespace.ID.String())
	rctx.URLParams.Add("name", "test-secret")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()

	handlers.GetK8sSecret(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, rec.Body.String(), "Kubernetes cluster not available")
}

func TestConvertToK8sSecretInfo(t *testing.T) {
	k8sSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
			CreationTimestamp: metav1.Time{
				Time: time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC),
			},
			Labels: map[string]string{
				"app": "myapp",
			},
			Annotations: map[string]string{
				"secret-manager.io/managed": "true",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"username": []byte("admin"),
			"password": []byte("secret123"),
		},
	}

	managedSecrets := map[string]bool{
		"test-secret": true,
	}

	result := convertToK8sSecretInfo(k8sSecret, managedSecrets)

	assert.Equal(t, "test-secret", result.Name)
	assert.Equal(t, "default", result.Namespace)
	assert.Equal(t, "Opaque", result.Type)
	assert.True(t, result.ManagedByGitOps)
	assert.Len(t, result.DataKeys, 2)
	assert.Contains(t, result.DataKeys, "username")
	assert.Contains(t, result.DataKeys, "password")
	assert.Equal(t, "myapp", result.Labels["app"])
	assert.Equal(t, "true", result.Annotations["secret-manager.io/managed"])
}

func TestConvertToK8sSecretInfo_UnmanagedSecret(t *testing.T) {
	k8sSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "unmanaged-secret",
			Namespace: "default",
			CreationTimestamp: metav1.Time{
				Time: time.Date(2026, 3, 20, 8, 0, 0, 0, time.UTC),
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"token": []byte("abc123"),
		},
	}

	managedSecrets := map[string]bool{} // Empty - no managed secrets

	result := convertToK8sSecretInfo(k8sSecret, managedSecrets)

	assert.Equal(t, "unmanaged-secret", result.Name)
	assert.False(t, result.ManagedByGitOps)
	assert.Len(t, result.DataKeys, 1)
	assert.Contains(t, result.DataKeys, "token")
}
