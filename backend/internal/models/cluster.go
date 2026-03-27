package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Cluster represents a Kubernetes cluster configuration
type Cluster struct {
	ID              uuid.UUID  `gorm:"type:uuid;primary_key" json:"id"`
	Name            string     `gorm:"uniqueIndex;not null" json:"name" binding:"required"`
	KubeconfigRef   string     `gorm:"column:kubeconfig_ref;not null" json:"kubeconfig_ref" binding:"required"`
	Environment     string     `gorm:"not null;check:environment IN ('development', 'staging', 'production')" json:"environment" binding:"required,oneof=development staging production"`
	IsHealthy       bool       `gorm:"default:true" json:"is_healthy"`
	LastHealthCheck *time.Time `json:"last_health_check,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`

	// Relationships
	Namespaces []Namespace `gorm:"foreignKey:ClusterID;constraint:OnDelete:CASCADE" json:"namespaces,omitempty"`
}

func (c *Cluster) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}

func (c *Cluster) TableName() string {
	return "clusters"
}
