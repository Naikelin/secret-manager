package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/yourorg/secret-manager/internal/drift"
	"github.com/yourorg/secret-manager/internal/models"
	"github.com/yourorg/secret-manager/pkg/logger"
	"gorm.io/gorm"
)

// DriftHandlers contains handlers for drift detection operations
type DriftHandlers struct {
	db            *gorm.DB
	driftDetector *drift.DriftDetector
}

// NewDriftHandlers creates a new DriftHandlers instance
func NewDriftHandlers(db *gorm.DB, driftDetector *drift.DriftDetector) *DriftHandlers {
	return &DriftHandlers{
		db:            db,
		driftDetector: driftDetector,
	}
}

// DriftCheckResponse represents the response for drift check operation
type DriftCheckResponse struct {
	Namespace string              `json:"namespace"`
	Checked   int                 `json:"checked"`
	Drifted   int                 `json:"drifted"`
	Events    []DriftEventSummary `json:"events"`
}

// DriftEventSummary represents a summarized drift event
type DriftEventSummary struct {
	ID          uuid.UUID `json:"id"`
	SecretName  string    `json:"secret_name"`
	DetectedAt  time.Time `json:"detected_at"`
	DiffSummary string    `json:"diff_summary"`
}

// DriftEventsResponse represents the response for listing drift events
type DriftEventsResponse struct {
	Namespace string             `json:"namespace"`
	Total     int64              `json:"total"`
	Events    []DriftEventDetail `json:"events"`
}

// DriftEventDetail represents a detailed drift event
type DriftEventDetail struct {
	ID               uuid.UUID  `json:"id"`
	SecretName       string     `json:"secret_name"`
	DetectedAt       time.Time  `json:"detected_at"`
	GitVersion       JSONObject `json:"git_version"`
	K8sVersion       JSONObject `json:"k8s_version"`
	Diff             JSONObject `json:"diff"`
	ResolvedAt       *time.Time `json:"resolved_at,omitempty"`
	ResolvedBy       *uuid.UUID `json:"resolved_by,omitempty"`
	ResolutionAction *string    `json:"resolution_action,omitempty"`
}

// JSONObject is a helper type for JSON fields
type JSONObject map[string]interface{}

// TriggerDriftCheck handles POST /api/v1/namespaces/{namespace}/drift-check
func (h *DriftHandlers) TriggerDriftCheck(w http.ResponseWriter, r *http.Request) {
	namespaceIDStr := chi.URLParam(r, "namespace")

	// Parse namespace ID
	namespaceID, err := uuid.Parse(namespaceIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid namespace ID")
		return
	}

	// Load namespace to get name
	var namespace models.Namespace
	if err := h.db.First(&namespace, namespaceID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(w, http.StatusNotFound, "Namespace not found")
		} else {
			logger.Error("Failed to fetch namespace", "error", err)
			respondError(w, http.StatusInternalServerError, "Failed to fetch namespace")
		}
		return
	}

	// Count published secrets in namespace
	var totalPublished int64
	if err := h.db.Model(&models.SecretDraft{}).Where("namespace_id = ? AND status = ?", namespaceID, "published").Count(&totalPublished).Error; err != nil {
		logger.Error("Failed to count published secrets", "error", err)
		respondError(w, http.StatusInternalServerError, "Failed to count secrets")
		return
	}

	// Run drift detection
	driftEvents, err := h.driftDetector.DetectDriftForNamespace(namespaceID)
	if err != nil {
		logger.Error("Drift detection failed", "namespace", namespace.Name, "error", err)
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("Drift detection failed: %v", err))
		return
	}

	// Convert drift events to summary format
	eventSummaries := make([]DriftEventSummary, 0, len(driftEvents))
	for _, event := range driftEvents {
		// Parse diff JSON to extract summary
		var diffData map[string]interface{}
		if err := json.Unmarshal([]byte(event.Diff), &diffData); err != nil {
			logger.Warn("Failed to parse drift diff JSON", "error", err)
			continue
		}

		// Extract summary from diff
		diffSummary := extractDiffSummary(diffData)

		eventSummaries = append(eventSummaries, DriftEventSummary{
			ID:          event.ID,
			SecretName:  event.SecretName,
			DetectedAt:  event.DetectedAt,
			DiffSummary: diffSummary,
		})
	}

	// Return response
	response := DriftCheckResponse{
		Namespace: namespace.Name,
		Checked:   int(totalPublished),
		Drifted:   len(driftEvents),
		Events:    eventSummaries,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// ListDriftEvents handles GET /api/v1/namespaces/{namespace}/drift-events
func (h *DriftHandlers) ListDriftEvents(w http.ResponseWriter, r *http.Request) {
	namespaceIDStr := chi.URLParam(r, "namespace")

	// Parse namespace ID
	namespaceID, err := uuid.Parse(namespaceIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid namespace ID")
		return
	}

	// Load namespace to get name
	var namespace models.Namespace
	if err := h.db.First(&namespace, namespaceID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(w, http.StatusNotFound, "Namespace not found")
		} else {
			logger.Error("Failed to fetch namespace", "error", err)
			respondError(w, http.StatusInternalServerError, "Failed to fetch namespace")
		}
		return
	}

	// Parse query parameters
	query := h.db.Model(&models.DriftEvent{}).Where("namespace_id = ?", namespaceID)

	// Filter by status (resolved/active)
	status := r.URL.Query().Get("status")
	if status == "resolved" {
		query = query.Where("resolved_at IS NOT NULL")
	} else if status == "active" {
		query = query.Where("resolved_at IS NULL")
	}

	// Filter by secret name
	secretName := r.URL.Query().Get("secret_name")
	if secretName != "" {
		query = query.Where("secret_name = ?", secretName)
	}

	// Count total matching events
	var total int64
	if err := query.Count(&total).Error; err != nil {
		logger.Error("Failed to count drift events", "error", err)
		respondError(w, http.StatusInternalServerError, "Failed to count drift events")
		return
	}

	// Pagination
	limit := 50
	offset := 0
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 100 {
			limit = parsedLimit
		}
	}
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	// Fetch drift events
	var driftEvents []models.DriftEvent
	if err := query.Order("detected_at DESC").Limit(limit).Offset(offset).Find(&driftEvents).Error; err != nil {
		logger.Error("Failed to fetch drift events", "error", err)
		respondError(w, http.StatusInternalServerError, "Failed to fetch drift events")
		return
	}

	// Convert to detailed format
	eventDetails := make([]DriftEventDetail, 0, len(driftEvents))
	for _, event := range driftEvents {
		// Parse JSON fields
		var gitVersion, k8sVersion, diff JSONObject
		json.Unmarshal([]byte(event.GitVersion), &gitVersion)
		json.Unmarshal([]byte(event.K8sVersion), &k8sVersion)
		json.Unmarshal([]byte(event.Diff), &diff)

		eventDetails = append(eventDetails, DriftEventDetail{
			ID:               event.ID,
			SecretName:       event.SecretName,
			DetectedAt:       event.DetectedAt,
			GitVersion:       gitVersion,
			K8sVersion:       k8sVersion,
			Diff:             diff,
			ResolvedAt:       event.ResolvedAt,
			ResolvedBy:       event.ResolvedBy,
			ResolutionAction: event.ResolutionAction,
		})
	}

	// Return response
	response := DriftEventsResponse{
		Namespace: namespace.Name,
		Total:     total,
		Events:    eventDetails,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// extractDiffSummary extracts a human-readable summary from drift diff data
func extractDiffSummary(diffData map[string]interface{}) string {
	// Check for error field (missing file/secret)
	if errorMsg, ok := diffData["error"].(string); ok {
		return errorMsg
	}

	// Check for differences array
	if differences, ok := diffData["differences"].([]interface{}); ok {
		if len(differences) == 0 {
			return "No differences detected"
		}
		if len(differences) == 1 {
			if diff, ok := differences[0].(string); ok {
				return diff
			}
		}
		return fmt.Sprintf("%d differences detected", len(differences))
	}

	return "Drift detected"
}

// CheckAllNamespaces handles POST /api/v1/drift/check-all
// Admin-only endpoint to manually trigger drift check across all namespaces
func (h *DriftHandlers) CheckAllNamespaces(w http.ResponseWriter, r *http.Request) {
	var namespaces []models.Namespace
	if err := h.db.Find(&namespaces).Error; err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to fetch namespaces")
		return
	}

	type NamespaceResult struct {
		NamespaceID   string `json:"namespace_id"`
		NamespaceName string `json:"namespace_name"`
		DriftCount    int    `json:"drift_count"`
		Error         string `json:"error,omitempty"`
	}

	results := []NamespaceResult{}

	for _, ns := range namespaces {
		events, err := h.driftDetector.DetectDriftForNamespace(ns.ID)

		result := NamespaceResult{
			NamespaceID:   ns.ID.String(),
			NamespaceName: ns.Name,
		}

		if err != nil {
			result.Error = err.Error()
		} else {
			result.DriftCount = len(events)
		}

		results = append(results, result)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"total_namespaces": len(namespaces),
		"results":          results,
	})
}

// DriftComparisonResponse represents side-by-side Git vs K8s comparison
type DriftComparisonResponse struct {
	GitData map[string]string `json:"git_data"`
	K8sData map[string]string `json:"k8s_data"`
}

// GetDriftComparison handles GET /api/v1/drift-events/{drift_id}/compare
// Returns side-by-side comparison of Git vs K8s secret data for visual diff
func (h *DriftHandlers) GetDriftComparison(w http.ResponseWriter, r *http.Request) {
	driftIDStr := chi.URLParam(r, "drift_id")

	// Parse drift ID
	driftID, err := uuid.Parse(driftIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid drift event ID")
		return
	}

	// Load drift event with namespace preloaded
	var event models.DriftEvent
	if err := h.db.Preload("Namespace").First(&event, driftID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(w, http.StatusNotFound, "Drift event not found")
		} else {
			logger.Error("Failed to fetch drift event", "error", err)
			respondError(w, http.StatusInternalServerError, "Failed to fetch drift event")
		}
		return
	}

	// Check if drift detector is available
	if h.driftDetector == nil {
		respondError(w, http.StatusServiceUnavailable, "Drift detector not configured")
		return
	}

	// Fetch Git and K8s data for comparison
	gitData, k8sData, err := h.driftDetector.GetComparisonData(event.Namespace.Name, event.SecretName)
	if err != nil {
		logger.Error("Failed to fetch comparison data", "error", err)
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to fetch comparison data: %v", err))
		return
	}

	// Return comparison response
	response := DriftComparisonResponse{
		GitData: gitData,
		K8sData: k8sData,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
