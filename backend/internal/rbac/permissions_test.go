package rbac

import (
	"testing"

	"github.com/google/uuid"
	"github.com/yourorg/secret-manager/internal/models"
)

func TestRoleHierarchy(t *testing.T) {
	tests := []struct {
		name     string
		role     Role
		level    int
		hasAdmin bool
	}{
		{
			name:     "viewer is level 1",
			role:     RoleViewer,
			level:    1,
			hasAdmin: false,
		},
		{
			name:     "editor is level 2",
			role:     RoleEditor,
			level:    2,
			hasAdmin: false,
		},
		{
			name:     "admin is level 3",
			role:     RoleAdmin,
			level:    3,
			hasAdmin: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level := roleHierarchy[tt.role]
			if level != tt.level {
				t.Errorf("expected level %d, got %d", tt.level, level)
			}

			if tt.hasAdmin && level != 3 {
				t.Error("admin should be highest level (3)")
			}
		})
	}
}

func TestCanReadSecret(t *testing.T) {
	namespaceID := uuid.New()
	otherNamespaceID := uuid.New()

	tests := []struct {
		name        string
		permissions []models.GroupPermission
		namespaceID uuid.UUID
		expected    bool
	}{
		{
			name: "viewer can read",
			permissions: []models.GroupPermission{
				{NamespaceID: namespaceID, Role: string(RoleViewer)},
			},
			namespaceID: namespaceID,
			expected:    true,
		},
		{
			name: "editor can read",
			permissions: []models.GroupPermission{
				{NamespaceID: namespaceID, Role: string(RoleEditor)},
			},
			namespaceID: namespaceID,
			expected:    true,
		},
		{
			name: "admin can read",
			permissions: []models.GroupPermission{
				{NamespaceID: namespaceID, Role: string(RoleAdmin)},
			},
			namespaceID: namespaceID,
			expected:    true,
		},
		{
			name:        "no permissions cannot read",
			permissions: []models.GroupPermission{},
			namespaceID: namespaceID,
			expected:    false,
		},
		{
			name: "permission in different namespace cannot read",
			permissions: []models.GroupPermission{
				{NamespaceID: otherNamespaceID, Role: string(RoleAdmin)},
			},
			namespaceID: namespaceID,
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CanReadSecret(tt.permissions, tt.namespaceID)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestCanWriteSecret(t *testing.T) {
	namespaceID := uuid.New()

	tests := []struct {
		name        string
		permissions []models.GroupPermission
		expected    bool
	}{
		{
			name: "viewer cannot write",
			permissions: []models.GroupPermission{
				{NamespaceID: namespaceID, Role: string(RoleViewer)},
			},
			expected: false,
		},
		{
			name: "editor can write",
			permissions: []models.GroupPermission{
				{NamespaceID: namespaceID, Role: string(RoleEditor)},
			},
			expected: true,
		},
		{
			name: "admin can write",
			permissions: []models.GroupPermission{
				{NamespaceID: namespaceID, Role: string(RoleAdmin)},
			},
			expected: true,
		},
		{
			name:        "no permissions cannot write",
			permissions: []models.GroupPermission{},
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CanWriteSecret(tt.permissions, namespaceID)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestCanPublishSecret(t *testing.T) {
	namespaceID := uuid.New()

	tests := []struct {
		name        string
		permissions []models.GroupPermission
		expected    bool
	}{
		{
			name: "viewer cannot publish",
			permissions: []models.GroupPermission{
				{NamespaceID: namespaceID, Role: string(RoleViewer)},
			},
			expected: false,
		},
		{
			name: "editor can publish",
			permissions: []models.GroupPermission{
				{NamespaceID: namespaceID, Role: string(RoleEditor)},
			},
			expected: true,
		},
		{
			name: "admin can publish",
			permissions: []models.GroupPermission{
				{NamespaceID: namespaceID, Role: string(RoleAdmin)},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CanPublishSecret(tt.permissions, namespaceID)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestCanDeleteSecret(t *testing.T) {
	namespaceID := uuid.New()

	tests := []struct {
		name        string
		permissions []models.GroupPermission
		expected    bool
	}{
		{
			name: "viewer cannot delete",
			permissions: []models.GroupPermission{
				{NamespaceID: namespaceID, Role: string(RoleViewer)},
			},
			expected: false,
		},
		{
			name: "editor cannot delete",
			permissions: []models.GroupPermission{
				{NamespaceID: namespaceID, Role: string(RoleEditor)},
			},
			expected: false,
		},
		{
			name: "admin can delete",
			permissions: []models.GroupPermission{
				{NamespaceID: namespaceID, Role: string(RoleAdmin)},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CanDeleteSecret(tt.permissions, namespaceID)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestCanManageNamespace(t *testing.T) {
	namespaceID := uuid.New()

	tests := []struct {
		name        string
		permissions []models.GroupPermission
		expected    bool
	}{
		{
			name: "viewer cannot manage",
			permissions: []models.GroupPermission{
				{NamespaceID: namespaceID, Role: string(RoleViewer)},
			},
			expected: false,
		},
		{
			name: "editor cannot manage",
			permissions: []models.GroupPermission{
				{NamespaceID: namespaceID, Role: string(RoleEditor)},
			},
			expected: false,
		},
		{
			name: "admin can manage",
			permissions: []models.GroupPermission{
				{NamespaceID: namespaceID, Role: string(RoleAdmin)},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CanManageNamespace(tt.permissions, namespaceID)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestMultipleGroupPermissions(t *testing.T) {
	namespace1 := uuid.New()
	namespace2 := uuid.New()
	namespace3 := uuid.New()

	permissions := []models.GroupPermission{
		{NamespaceID: namespace1, Role: string(RoleViewer)},
		{NamespaceID: namespace2, Role: string(RoleEditor)},
		{NamespaceID: namespace3, Role: string(RoleAdmin)},
	}

	tests := []struct {
		name        string
		checkFunc   func([]models.GroupPermission, uuid.UUID) bool
		namespaceID uuid.UUID
		expected    bool
		description string
	}{
		{
			name:        "viewer in ns1 can read",
			checkFunc:   CanReadSecret,
			namespaceID: namespace1,
			expected:    true,
			description: "viewer role allows reading",
		},
		{
			name:        "viewer in ns1 cannot write",
			checkFunc:   CanWriteSecret,
			namespaceID: namespace1,
			expected:    false,
			description: "viewer role does not allow writing",
		},
		{
			name:        "editor in ns2 can write",
			checkFunc:   CanWriteSecret,
			namespaceID: namespace2,
			expected:    true,
			description: "editor role allows writing",
		},
		{
			name:        "editor in ns2 cannot delete",
			checkFunc:   CanDeleteSecret,
			namespaceID: namespace2,
			expected:    false,
			description: "editor role does not allow deleting",
		},
		{
			name:        "admin in ns3 can delete",
			checkFunc:   CanDeleteSecret,
			namespaceID: namespace3,
			expected:    true,
			description: "admin role allows deleting",
		},
		{
			name:        "admin in ns3 can manage",
			checkFunc:   CanManageNamespace,
			namespaceID: namespace3,
			expected:    true,
			description: "admin role allows managing namespace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.checkFunc(permissions, tt.namespaceID)
			if result != tt.expected {
				t.Errorf("%s: expected %v, got %v", tt.description, tt.expected, result)
			}
		})
	}
}

func TestHighestRoleWins(t *testing.T) {
	namespaceID := uuid.New()

	// User has multiple groups with different roles in the same namespace
	// The highest role should apply
	permissions := []models.GroupPermission{
		{NamespaceID: namespaceID, Role: string(RoleViewer)},
		{NamespaceID: namespaceID, Role: string(RoleAdmin)},
		{NamespaceID: namespaceID, Role: string(RoleEditor)},
	}

	tests := []struct {
		name      string
		checkFunc func([]models.GroupPermission, uuid.UUID) bool
		expected  bool
	}{
		{
			name:      "can read with highest role",
			checkFunc: CanReadSecret,
			expected:  true,
		},
		{
			name:      "can write with highest role",
			checkFunc: CanWriteSecret,
			expected:  true,
		},
		{
			name:      "can publish with highest role",
			checkFunc: CanPublishSecret,
			expected:  true,
		},
		{
			name:      "can delete with highest role (admin)",
			checkFunc: CanDeleteSecret,
			expected:  true,
		},
		{
			name:      "can manage with highest role (admin)",
			checkFunc: CanManageNamespace,
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.checkFunc(permissions, namespaceID)
			if result != tt.expected {
				t.Errorf("expected %v, got %v - highest role should grant permission", tt.expected, result)
			}
		})
	}
}
