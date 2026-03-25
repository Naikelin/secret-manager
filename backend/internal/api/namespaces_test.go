package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourorg/secret-manager/internal/models"
)

// TestListNamespaces_Success tests successful namespace listing
func TestListNamespaces_Success(t *testing.T) {
	db := setupTestDB(t)

	// Create test user
	user := models.User{
		ID:    uuid.New(),
		Email: "test@example.com",
		Name:  "Test User",
	}
	require.NoError(t, db.Create(&user).Error)

	// Create test group
	group := models.Group{
		ID:   uuid.New(),
		Name: "test-group",
	}
	require.NoError(t, db.Create(&group).Error)

	// Associate user with group
	err := db.Exec("INSERT INTO user_groups (user_id, group_id) VALUES (?, ?)", user.ID, group.ID).Error
	require.NoError(t, err)

	// Create test namespaces
	namespace1 := models.Namespace{
		ID:          uuid.New(),
		Name:        "dev-namespace",
		Cluster:     "dev-cluster",
		Environment: "dev",
	}
	require.NoError(t, db.Create(&namespace1).Error)

	namespace2 := models.Namespace{
		ID:          uuid.New(),
		Name:        "staging-namespace",
		Cluster:     "staging-cluster",
		Environment: "staging",
	}
	require.NoError(t, db.Create(&namespace2).Error)

	namespace3 := models.Namespace{
		ID:          uuid.New(),
		Name:        "prod-namespace",
		Cluster:     "prod-cluster",
		Environment: "prod",
	}
	require.NoError(t, db.Create(&namespace3).Error)

	// Create permissions for group (access to namespace1 and namespace2 only)
	permission1 := models.GroupPermission{
		ID:          uuid.New(),
		GroupID:     group.ID,
		NamespaceID: namespace1.ID,
		Role:        "editor",
	}
	require.NoError(t, db.Create(&permission1).Error)

	permission2 := models.GroupPermission{
		ID:          uuid.New(),
		GroupID:     group.ID,
		NamespaceID: namespace2.ID,
		Role:        "viewer",
	}
	require.NoError(t, db.Create(&permission2).Error)

	// Create handlers
	handlers := NewNamespaceHandlers(db)

	// Create request with user context
	req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces", nil)
	req = req.WithContext(context.WithValue(req.Context(), "user_id", user.ID))

	// Execute request
	rr := httptest.NewRecorder()
	handlers.ListNamespaces(rr, req)

	// Assert response
	assert.Equal(t, http.StatusOK, rr.Code)

	var namespaces []models.Namespace
	err = json.NewDecoder(rr.Body).Decode(&namespaces)
	require.NoError(t, err)

	// Should only return namespaces the user has access to (2 out of 3)
	assert.Len(t, namespaces, 2)

	// Verify namespaces are sorted by name
	assert.Equal(t, "dev-namespace", namespaces[0].Name)
	assert.Equal(t, "staging-namespace", namespaces[1].Name)
}

// TestListNamespaces_SingleNamespace tests user with access to single namespace
func TestListNamespaces_SingleNamespace(t *testing.T) {
	db := setupTestDB(t)

	// Create test user
	user := models.User{
		ID:    uuid.New(),
		Email: "test@example.com",
		Name:  "Test User",
	}
	require.NoError(t, db.Create(&user).Error)

	// Create test group
	group := models.Group{
		ID:   uuid.New(),
		Name: "test-group",
	}
	require.NoError(t, db.Create(&group).Error)

	// Associate user with group
	err := db.Exec("INSERT INTO user_groups (user_id, group_id) VALUES (?, ?)", user.ID, group.ID).Error
	require.NoError(t, err)

	// Create single test namespace
	namespace := models.Namespace{
		ID:          uuid.New(),
		Name:        "dev-namespace",
		Cluster:     "dev-cluster",
		Environment: "dev",
	}
	require.NoError(t, db.Create(&namespace).Error)

	// Create permission for group
	permission := models.GroupPermission{
		ID:          uuid.New(),
		GroupID:     group.ID,
		NamespaceID: namespace.ID,
		Role:        "viewer",
	}
	require.NoError(t, db.Create(&permission).Error)

	// Create handlers
	handlers := NewNamespaceHandlers(db)

	// Create request with user context
	req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces", nil)
	req = req.WithContext(context.WithValue(req.Context(), "user_id", user.ID))

	// Execute request
	rr := httptest.NewRecorder()
	handlers.ListNamespaces(rr, req)

	// Assert response
	assert.Equal(t, http.StatusOK, rr.Code)

	var namespaces []models.Namespace
	err = json.NewDecoder(rr.Body).Decode(&namespaces)
	require.NoError(t, err)

	assert.Len(t, namespaces, 1)
	assert.Equal(t, "dev-namespace", namespaces[0].Name)
}

// TestListNamespaces_NoAccess tests user with no namespace access
func TestListNamespaces_NoAccess(t *testing.T) {
	db := setupTestDB(t)

	// Create test user
	user := models.User{
		ID:    uuid.New(),
		Email: "test@example.com",
		Name:  "Test User",
	}
	require.NoError(t, db.Create(&user).Error)

	// Create test group (but no permissions)
	group := models.Group{
		ID:   uuid.New(),
		Name: "test-group",
	}
	require.NoError(t, db.Create(&group).Error)

	// Associate user with group
	err := db.Exec("INSERT INTO user_groups (user_id, group_id) VALUES (?, ?)", user.ID, group.ID).Error
	require.NoError(t, err)

	// Create handlers
	handlers := NewNamespaceHandlers(db)

	// Create request with user context
	req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces", nil)
	req = req.WithContext(context.WithValue(req.Context(), "user_id", user.ID))

	// Execute request
	rr := httptest.NewRecorder()
	handlers.ListNamespaces(rr, req)

	// Assert response
	assert.Equal(t, http.StatusOK, rr.Code)

	var namespaces []models.Namespace
	err = json.NewDecoder(rr.Body).Decode(&namespaces)
	require.NoError(t, err)

	// Should return empty list
	assert.Len(t, namespaces, 0)
}

// TestListNamespaces_Unauthorized tests request without JWT
func TestListNamespaces_Unauthorized(t *testing.T) {
	db := setupTestDB(t)

	// Create handlers
	handlers := NewNamespaceHandlers(db)

	// Create request WITHOUT user context
	req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces", nil)

	// Execute request
	rr := httptest.NewRecorder()
	handlers.ListNamespaces(rr, req)

	// Assert 401 response
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// TestListNamespaces_MultipleGroups tests user in multiple groups
func TestListNamespaces_MultipleGroups(t *testing.T) {
	db := setupTestDB(t)

	// Create test user
	user := models.User{
		ID:    uuid.New(),
		Email: "test@example.com",
		Name:  "Test User",
	}
	require.NoError(t, db.Create(&user).Error)

	// Create test groups
	group1 := models.Group{
		ID:   uuid.New(),
		Name: "group1",
	}
	require.NoError(t, db.Create(&group1).Error)

	group2 := models.Group{
		ID:   uuid.New(),
		Name: "group2",
	}
	require.NoError(t, db.Create(&group2).Error)

	// Associate user with both groups
	err := db.Exec("INSERT INTO user_groups (user_id, group_id) VALUES (?, ?)", user.ID, group1.ID).Error
	require.NoError(t, err)
	err = db.Exec("INSERT INTO user_groups (user_id, group_id) VALUES (?, ?)", user.ID, group2.ID).Error
	require.NoError(t, err)

	// Create test namespaces
	namespace1 := models.Namespace{
		ID:          uuid.New(),
		Name:        "namespace1",
		Cluster:     "cluster1",
		Environment: "dev",
	}
	require.NoError(t, db.Create(&namespace1).Error)

	namespace2 := models.Namespace{
		ID:          uuid.New(),
		Name:        "namespace2",
		Cluster:     "cluster2",
		Environment: "staging",
	}
	require.NoError(t, db.Create(&namespace2).Error)

	namespace3 := models.Namespace{
		ID:          uuid.New(),
		Name:        "namespace3",
		Cluster:     "cluster3",
		Environment: "prod",
	}
	require.NoError(t, db.Create(&namespace3).Error)

	// Group1 has access to namespace1 and namespace2
	permission1 := models.GroupPermission{
		ID:          uuid.New(),
		GroupID:     group1.ID,
		NamespaceID: namespace1.ID,
		Role:        "editor",
	}
	require.NoError(t, db.Create(&permission1).Error)

	permission2 := models.GroupPermission{
		ID:          uuid.New(),
		GroupID:     group1.ID,
		NamespaceID: namespace2.ID,
		Role:        "viewer",
	}
	require.NoError(t, db.Create(&permission2).Error)

	// Group2 has access to namespace2 (overlapping) and namespace3
	permission3 := models.GroupPermission{
		ID:          uuid.New(),
		GroupID:     group2.ID,
		NamespaceID: namespace2.ID,
		Role:        "admin",
	}
	require.NoError(t, db.Create(&permission3).Error)

	permission4 := models.GroupPermission{
		ID:          uuid.New(),
		GroupID:     group2.ID,
		NamespaceID: namespace3.ID,
		Role:        "editor",
	}
	require.NoError(t, db.Create(&permission4).Error)

	// Create handlers
	handlers := NewNamespaceHandlers(db)

	// Create request with user context
	req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces", nil)
	req = req.WithContext(context.WithValue(req.Context(), "user_id", user.ID))

	// Execute request
	rr := httptest.NewRecorder()
	handlers.ListNamespaces(rr, req)

	// Assert response
	assert.Equal(t, http.StatusOK, rr.Code)

	var namespaces []models.Namespace
	err = json.NewDecoder(rr.Body).Decode(&namespaces)
	require.NoError(t, err)

	// Should return all 3 namespaces (DISTINCT removes duplicates)
	assert.Len(t, namespaces, 3)

	// Verify namespaces are sorted by name
	assert.Equal(t, "namespace1", namespaces[0].Name)
	assert.Equal(t, "namespace2", namespaces[1].Name)
	assert.Equal(t, "namespace3", namespaces[2].Name)
}
