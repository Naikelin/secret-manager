package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// User represents a cached OAuth2 user
type User struct {
	ID         uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	Email      string    `gorm:"uniqueIndex;not null" json:"email"`
	Name       string    `gorm:"not null" json:"name"`
	AzureADOID string    `gorm:"column:azure_ad_oid" json:"azure_ad_oid,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`

	// Relationships
	Groups []Group `gorm:"many2many:user_groups;" json:"groups,omitempty"`
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

// Group represents an Azure AD group or local group
type Group struct {
	ID         uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	Name       string    `gorm:"uniqueIndex;not null" json:"name"`
	AzureADGID string    `gorm:"column:azure_ad_gid" json:"azure_ad_gid,omitempty"`
	CreatedAt  time.Time `json:"created_at"`

	// Relationships
	Users       []User            `gorm:"many2many:user_groups;" json:"users,omitempty"`
	Permissions []GroupPermission `gorm:"foreignKey:GroupID" json:"permissions,omitempty"`
}

func (g *Group) BeforeCreate(tx *gorm.DB) error {
	if g.ID == uuid.Nil {
		g.ID = uuid.New()
	}
	return nil
}

// Namespace represents a Kubernetes namespace
type Namespace struct {
	ID          uuid.UUID  `gorm:"type:uuid;primary_key" json:"id"`
	Name        string     `gorm:"not null" json:"name"`
	ClusterID   *uuid.UUID `gorm:"type:uuid;index:idx_namespaces_cluster_id" json:"cluster_id,omitempty"`
	Cluster     string     `gorm:"not null" json:"cluster"` // Deprecated: will be removed after migration
	Environment string     `gorm:"not null;check:environment IN ('dev', 'staging', 'prod')" json:"environment"`
	CreatedAt   time.Time  `gorm:"" json:"created_at"`

	// Relationships
	ClusterRef   *Cluster          `gorm:"foreignKey:ClusterID;constraint:OnDelete:CASCADE" json:"cluster_ref,omitempty"`
	Permissions  []GroupPermission `gorm:"foreignKey:NamespaceID" json:"permissions,omitempty"`
	SecretDrafts []SecretDraft     `gorm:"foreignKey:NamespaceID" json:"secret_drafts,omitempty"`
	DriftEvents  []DriftEvent      `gorm:"foreignKey:NamespaceID" json:"drift_events,omitempty"`
}

func (n *Namespace) BeforeCreate(tx *gorm.DB) error {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	return nil
}

func (n *Namespace) TableName() string {
	return "namespaces"
}

// GroupPermission represents RBAC permissions
type GroupPermission struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	GroupID     uuid.UUID `gorm:"type:uuid;not null;index:idx_group_permissions_group" json:"group_id"`
	NamespaceID uuid.UUID `gorm:"type:uuid;not null;index:idx_group_permissions_namespace" json:"namespace_id"`
	Role        string    `gorm:"not null;check:role IN ('viewer', 'editor', 'admin')" json:"role"`
	CreatedAt   time.Time `json:"created_at"`

	// Relationships
	Group     Group     `gorm:"foreignKey:GroupID" json:"group,omitempty"`
	Namespace Namespace `gorm:"foreignKey:NamespaceID" json:"namespace,omitempty"`
}

func (gp *GroupPermission) BeforeCreate(tx *gorm.DB) error {
	if gp.ID == uuid.Nil {
		gp.ID = uuid.New()
	}
	return nil
}

func (gp *GroupPermission) TableName() string {
	return "group_permissions"
}
