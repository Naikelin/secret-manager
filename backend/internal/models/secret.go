package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// SecretDraft represents a secret in the staging area
type SecretDraft struct {
	ID          uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	SecretName  string         `gorm:"not null;index:idx_secret_drafts_secret_namespace" json:"secret_name"`
	NamespaceID uuid.UUID      `gorm:"type:uuid;not null;index:idx_secret_drafts_secret_namespace" json:"namespace_id"`
	Data        datatypes.JSON `gorm:"type:jsonb;not null" json:"data"`
	Status      string         `gorm:"not null;index:idx_secret_drafts_status;check:status IN ('draft', 'published', 'drifted')" json:"status"`
	GitBaseSHA  string         `gorm:"column:git_base_sha;size:64" json:"git_base_sha,omitempty"`
	EditedBy    *uuid.UUID     `gorm:"type:uuid;index:idx_secret_drafts_edited_by" json:"edited_by,omitempty"`
	EditedAt    time.Time      `gorm:"index:idx_secret_drafts_auto_discard" json:"edited_at"`
	PublishedBy *uuid.UUID     `gorm:"type:uuid" json:"published_by,omitempty"`
	PublishedAt *time.Time     `json:"published_at,omitempty"`
	CommitSHA   string         `gorm:"column:commit_sha;size:64" json:"commit_sha,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`

	// Relationships
	Namespace Namespace `gorm:"foreignKey:NamespaceID" json:"namespace,omitempty"`
	Editor    *User     `gorm:"foreignKey:EditedBy" json:"editor,omitempty"`
	Publisher *User     `gorm:"foreignKey:PublishedBy" json:"publisher,omitempty"`
}

func (sd *SecretDraft) BeforeCreate(tx *gorm.DB) error {
	if sd.ID == uuid.Nil {
		sd.ID = uuid.New()
	}
	return nil
}

func (sd *SecretDraft) TableName() string {
	return "secret_drafts"
}

// DriftEvent represents a Git vs K8s mismatch
type DriftEvent struct {
	ID               uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	SecretName       string         `gorm:"not null;index:idx_drift_events_secret_namespace" json:"secret_name"`
	NamespaceID      uuid.UUID      `gorm:"type:uuid;not null;index:idx_drift_events_secret_namespace" json:"namespace_id"`
	DetectedAt       time.Time      `gorm:"index:idx_drift_events_detected_at" json:"detected_at"`
	GitVersion       datatypes.JSON `gorm:"type:jsonb;not null" json:"git_version"`
	K8sVersion       datatypes.JSON `gorm:"type:jsonb;not null;column:k8s_version" json:"k8s_version"`
	Diff             datatypes.JSON `gorm:"type:jsonb;not null" json:"diff"`
	ResolvedAt       *time.Time     `json:"resolved_at,omitempty"`
	ResolvedBy       *uuid.UUID     `gorm:"type:uuid" json:"resolved_by,omitempty"`
	ResolutionAction string         `gorm:"check:resolution_action IN ('sync_from_git', 'import_to_git', 'ignore')" json:"resolution_action,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`

	// Relationships
	Namespace Namespace `gorm:"foreignKey:NamespaceID" json:"namespace,omitempty"`
	Resolver  *User     `gorm:"foreignKey:ResolvedBy" json:"resolver,omitempty"`
}

func (de *DriftEvent) BeforeCreate(tx *gorm.DB) error {
	if de.ID == uuid.Nil {
		de.ID = uuid.New()
	}
	return nil
}

func (de *DriftEvent) TableName() string {
	return "drift_events"
}

// AuditLog represents an audit trail entry
type AuditLog struct {
	ID           uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID       *uuid.UUID     `gorm:"type:uuid;index:idx_audit_logs_user_timestamp" json:"user_id,omitempty"`
	ActionType   string         `gorm:"not null" json:"action_type"`
	ResourceType string         `gorm:"not null" json:"resource_type"`
	ResourceName string         `gorm:"not null;index:idx_audit_logs_resource_timestamp" json:"resource_name"`
	NamespaceID  *uuid.UUID     `gorm:"type:uuid;index:idx_audit_logs_namespace_timestamp" json:"namespace_id,omitempty"`
	Timestamp    time.Time      `gorm:"index:idx_audit_logs_user_timestamp,priority:2;index:idx_audit_logs_resource_timestamp,priority:2;index:idx_audit_logs_namespace_timestamp,priority:2" json:"timestamp"`
	Metadata     datatypes.JSON `gorm:"type:jsonb" json:"metadata,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`

	// Relationships
	User      *User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Namespace *Namespace `gorm:"foreignKey:NamespaceID" json:"namespace,omitempty"`
}

func (al *AuditLog) BeforeCreate(tx *gorm.DB) error {
	if al.ID == uuid.Nil {
		al.ID = uuid.New()
	}
	return nil
}

func (al *AuditLog) TableName() string {
	return "audit_logs"
}
