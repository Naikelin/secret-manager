package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/yourorg/secret-manager/internal/drift"
	"github.com/yourorg/secret-manager/internal/middleware"
	"gorm.io/gorm"
)

type DriftResolutionHandlers struct {
	db            *gorm.DB
	driftDetector *drift.DriftDetector
}

func NewDriftResolutionHandlers(db *gorm.DB, detector *drift.DriftDetector) *DriftResolutionHandlers {
	return &DriftResolutionHandlers{
		db:            db,
		driftDetector: detector,
	}
}

// SyncFromGit overwrites K8s secret with Git version (Git is source of truth)
func (h *DriftResolutionHandlers) SyncFromGit(w http.ResponseWriter, r *http.Request) {
	_, err := middleware.GetUserFromContext(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	driftIDStr := chi.URLParam(r, "drift_id")
	driftID, err := uuid.Parse(driftIDStr)
	if err != nil {
		http.Error(w, "invalid drift event ID", http.StatusBadRequest)
		return
	}

	if h.driftDetector == nil {
		http.Error(w, "drift detector not available", http.StatusServiceUnavailable)
		return
	}

	err = h.driftDetector.SyncFromGit(r.Context(), driftID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Secret synced from Git to K8s",
	})
}

// ImportToGit imports K8s secret to Git (K8s is source of truth)
func (h *DriftResolutionHandlers) ImportToGit(w http.ResponseWriter, r *http.Request) {
	_, err := middleware.GetUserFromContext(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	driftIDStr := chi.URLParam(r, "drift_id")
	driftID, err := uuid.Parse(driftIDStr)
	if err != nil {
		http.Error(w, "invalid drift event ID", http.StatusBadRequest)
		return
	}

	if h.driftDetector == nil {
		http.Error(w, "drift detector not available", http.StatusServiceUnavailable)
		return
	}

	err = h.driftDetector.ImportToGit(r.Context(), driftID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Secret imported from K8s to Git",
	})
}

// MarkResolved marks drift as manually resolved
func (h *DriftResolutionHandlers) MarkResolved(w http.ResponseWriter, r *http.Request) {
	user, err := middleware.GetUserFromContext(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	driftIDStr := chi.URLParam(r, "drift_id")
	driftID, err := uuid.Parse(driftIDStr)
	if err != nil {
		http.Error(w, "invalid drift event ID", http.StatusBadRequest)
		return
	}

	if h.driftDetector == nil {
		http.Error(w, "drift detector not available", http.StatusServiceUnavailable)
		return
	}

	err = h.driftDetector.MarkResolved(r.Context(), driftID, user.UserID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Drift marked as resolved",
	})
}
