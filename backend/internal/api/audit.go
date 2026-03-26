package api

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/yourorg/secret-manager/internal/models"
	"gorm.io/gorm"
)

type AuditHandlers struct {
	db *gorm.DB
}

func NewAuditHandlers(db *gorm.DB) *AuditHandlers {
	return &AuditHandlers{db: db}
}

// AuditLogResponse represents the API response for audit logs
type AuditLogResponse struct {
	Total int               `json:"total"`
	Page  int               `json:"page"`
	Limit int               `json:"limit"`
	Logs  []models.AuditLog `json:"logs"`
}

// ListAuditLogs returns audit logs with filtering and pagination
// Query parameters:
//   - user_id: Filter by user UUID
//   - action: Filter by action type (e.g., "secret_publish", "drift_sync_from_git")
//   - resource_type: Filter by resource type (e.g., "secret", "namespace")
//   - resource_name: Filter by resource name
//   - namespace_id: Filter by namespace UUID
//   - start_date: Filter logs after this date (RFC3339 format)
//   - end_date: Filter logs before this date (RFC3339 format)
//   - page: Page number (default: 1)
//   - limit: Results per page (default: 50, max: 500)
//
// @Summary List audit logs
// @Description Get audit logs with filtering and pagination
// @Tags audit
// @Accept json
// @Produce json
// @Param user_id query string false "Filter by user ID (UUID)"
// @Param action query string false "Filter by action type"
// @Param resource_type query string false "Filter by resource type"
// @Param resource_name query string false "Filter by resource name"
// @Param namespace_id query string false "Filter by namespace ID (UUID)"
// @Param start_date query string false "Filter logs after date (RFC3339)"
// @Param end_date query string false "Filter logs before date (RFC3339)"
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Results per page (max 500)" default(50)
// @Success 200 {object} AuditLogResponse "Audit logs"
// @Failure 500 {object} map[string]string "Server error"
// @Security BearerAuth
// @Router /audit-logs [get]
func (h *AuditHandlers) ListAuditLogs(w http.ResponseWriter, r *http.Request) {
	query := h.db.Model(&models.AuditLog{})

	// Apply filters
	if userID := r.URL.Query().Get("user_id"); userID != "" {
		if uid, err := uuid.Parse(userID); err == nil {
			query = query.Where("user_id = ?", uid)
		}
	}

	if action := r.URL.Query().Get("action"); action != "" {
		query = query.Where("action_type = ?", action)
	}

	if resourceType := r.URL.Query().Get("resource_type"); resourceType != "" {
		query = query.Where("resource_type = ?", resourceType)
	}

	if resourceName := r.URL.Query().Get("resource_name"); resourceName != "" {
		query = query.Where("resource_name = ?", resourceName)
	}

	if namespaceID := r.URL.Query().Get("namespace_id"); namespaceID != "" {
		if nsID, err := uuid.Parse(namespaceID); err == nil {
			query = query.Where("namespace_id = ?", nsID)
		}
	}

	if startDate := r.URL.Query().Get("start_date"); startDate != "" {
		if t, err := time.Parse(time.RFC3339, startDate); err == nil {
			query = query.Where("timestamp >= ?", t)
		}
	}

	if endDate := r.URL.Query().Get("end_date"); endDate != "" {
		if t, err := time.Parse(time.RFC3339, endDate); err == nil {
			query = query.Where("timestamp <= ?", t)
		}
	}

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		http.Error(w, "failed to count audit logs", http.StatusInternalServerError)
		return
	}

	// Pagination
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 {
		limit = 50
	}
	if limit > 500 {
		limit = 500 // Max limit
	}

	offset := (page - 1) * limit

	// Fetch logs
	var logs []models.AuditLog
	if err := query.
		Order("timestamp DESC").
		Offset(offset).
		Limit(limit).
		Find(&logs).Error; err != nil {
		http.Error(w, "failed to fetch audit logs", http.StatusInternalServerError)
		return
	}

	response := AuditLogResponse{
		Total: int(total),
		Page:  page,
		Limit: limit,
		Logs:  logs,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ExportAuditLogsCSV exports audit logs as CSV with the same filtering options
// @Summary Export audit logs to CSV
// @Description Export audit logs as CSV file with same filtering options as list endpoint (max 10,000 rows)
// @Tags audit
// @Accept json
// @Produce text/csv
// @Param user_id query string false "Filter by user ID (UUID)"
// @Param action query string false "Filter by action type"
// @Param resource_type query string false "Filter by resource type"
// @Param resource_name query string false "Filter by resource name"
// @Param namespace_id query string false "Filter by namespace ID (UUID)"
// @Param start_date query string false "Filter logs after date (RFC3339)"
// @Param end_date query string false "Filter logs before date (RFC3339)"
// @Success 200 {file} string "CSV file download"
// @Failure 500 {object} map[string]string "Server error"
// @Security BearerAuth
// @Router /audit-logs/export [get]
func (h *AuditHandlers) ExportAuditLogsCSV(w http.ResponseWriter, r *http.Request) {
	query := h.db.Model(&models.AuditLog{})

	// Apply same filters as ListAuditLogs
	if userID := r.URL.Query().Get("user_id"); userID != "" {
		if uid, err := uuid.Parse(userID); err == nil {
			query = query.Where("user_id = ?", uid)
		}
	}

	if action := r.URL.Query().Get("action"); action != "" {
		query = query.Where("action_type = ?", action)
	}

	if resourceType := r.URL.Query().Get("resource_type"); resourceType != "" {
		query = query.Where("resource_type = ?", resourceType)
	}

	if resourceName := r.URL.Query().Get("resource_name"); resourceName != "" {
		query = query.Where("resource_name = ?", resourceName)
	}

	if namespaceID := r.URL.Query().Get("namespace_id"); namespaceID != "" {
		if nsID, err := uuid.Parse(namespaceID); err == nil {
			query = query.Where("namespace_id = ?", nsID)
		}
	}

	if startDate := r.URL.Query().Get("start_date"); startDate != "" {
		if t, err := time.Parse(time.RFC3339, startDate); err == nil {
			query = query.Where("timestamp >= ?", t)
		}
	}

	if endDate := r.URL.Query().Get("end_date"); endDate != "" {
		if t, err := time.Parse(time.RFC3339, endDate); err == nil {
			query = query.Where("timestamp <= ?", t)
		}
	}

	// Limit export to 10,000 rows for safety
	var logs []models.AuditLog
	if err := query.
		Order("timestamp DESC").
		Limit(10000).
		Find(&logs).Error; err != nil {
		http.Error(w, "failed to fetch audit logs", http.StatusInternalServerError)
		return
	}

	// Set CSV headers
	filename := fmt.Sprintf("audit-logs-%s.csv", time.Now().Format("2006-01-02"))
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	// Write CSV
	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Header row
	writer.Write([]string{
		"ID",
		"User ID",
		"Action Type",
		"Resource Type",
		"Resource Name",
		"Namespace ID",
		"Timestamp",
		"Metadata",
	})

	// Data rows
	for _, log := range logs {
		userID := ""
		if log.UserID != nil {
			userID = log.UserID.String()
		}

		namespaceID := ""
		if log.NamespaceID != nil {
			namespaceID = log.NamespaceID.String()
		}

		metadata := ""
		if log.Metadata != nil {
			metadataBytes, _ := json.Marshal(log.Metadata)
			metadata = string(metadataBytes)
		}

		writer.Write([]string{
			log.ID.String(),
			userID,
			log.ActionType,
			log.ResourceType,
			log.ResourceName,
			namespaceID,
			log.Timestamp.Format(time.RFC3339),
			metadata,
		})
	}
}
