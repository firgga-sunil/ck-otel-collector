// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter"

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// CleanupRequest represents a cleanup request
type CleanupRequest struct {
	Type    string            `json:"type"`    // "labels", "name", "expired"
	Filters map[string]string `json:"filters"` // label filters for type="labels"
	Pattern string            `json:"pattern"` // name pattern for type="name"
}

// CleanupResponse represents the cleanup response
type CleanupResponse struct {
	Success      bool   `json:"success"`
	DeletedCount int    `json:"deleted_count"`
	Message      string `json:"message"`
	Timestamp    string `json:"timestamp"`
}

// CleanupAPI provides HTTP endpoints for metric cleanup
type CleanupAPI struct {
	exporter *prometheusExporter
	logger   *zap.Logger
}

// NewCleanupAPI creates a new cleanup API instance
func NewCleanupAPI(exporter *prometheusExporter, logger *zap.Logger) *CleanupAPI {
	return &CleanupAPI{
		exporter: exporter,
		logger:   logger,
	}
}

// CleanupHandler handles HTTP cleanup requests
func (api *CleanupAPI) CleanupHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.writeErrorResponse(w, http.StatusMethodNotAllowed, "Only POST method is allowed")
		return
	}

	var req CleanupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.writeErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("Invalid JSON: %v", err))
		return
	}

	var deletedCount int

	switch req.Type {
	case "labels":
		if len(req.Filters) == 0 {
			api.writeErrorResponse(w, http.StatusBadRequest, "Filters are required for label-based cleanup")
			return
		}
		deletedCount = api.exporter.CleanByLabels(req.Filters)
		api.logger.Info("Cleanup by labels completed",
			zap.Any("filters", req.Filters),
			zap.Int("deleted_count", deletedCount))

	case "name":
		if req.Pattern == "" {
			api.writeErrorResponse(w, http.StatusBadRequest, "Pattern is required for name-based cleanup")
			return
		}
		deletedCount = api.exporter.CleanByMetricName(req.Pattern)
		api.logger.Info("Cleanup by name completed",
			zap.String("pattern", req.Pattern),
			zap.Int("deleted_count", deletedCount))

	case "expired":
		deletedCount = api.exporter.CleanExpired()
		api.logger.Info("Cleanup expired metrics completed",
			zap.Int("deleted_count", deletedCount))

	default:
		api.writeErrorResponse(w, http.StatusBadRequest,
			"Invalid cleanup type. Supported types: 'labels', 'name', 'expired'")
		return
	}

	response := CleanupResponse{
		Success:      true,
		DeletedCount: deletedCount,
		Message:      fmt.Sprintf("Successfully deleted %d metrics", deletedCount),
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// StatusHandler provides cleanup status and available operations
func (api *CleanupAPI) StatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.writeErrorResponse(w, http.StatusMethodNotAllowed, "Only GET method is allowed")
		return
	}

	status := map[string]interface{}{
		"cleanup_api_version":  "1.0",
		"supported_operations": []string{"labels", "name", "expired"},
		"timestamp":            time.Now().UTC().Format(time.RFC3339),
		"endpoints": map[string]string{
			"cleanup": "/cleanup",
			"status":  "/cleanup/status",
		},
		"examples": map[string]interface{}{
			"cleanup_by_labels": CleanupRequest{
				Type: "labels",
				Filters: map[string]string{
					"job":      "test-job",
					"instance": "test-instance",
				},
			},
			"cleanup_by_name": CleanupRequest{
				Type:    "name",
				Pattern: "test_metric_.*",
			},
			"cleanup_expired": CleanupRequest{
				Type: "expired",
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(status)
}

// MetricsHandler provides metrics about cleanup operations
func (api *CleanupAPI) MetricsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.writeErrorResponse(w, http.StatusMethodNotAllowed, "Only GET method is allowed")
		return
	}

	// Count current metrics
	metrics, _, _, _, _, _ := api.exporter.collector.accumulator.Collect()
	currentCount := len(metrics)

	response := map[string]interface{}{
		"current_metric_count": currentCount,
		"timestamp":            time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// writeErrorResponse writes an error response
func (api *CleanupAPI) writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	api.logger.Error("Cleanup API error", zap.String("message", message), zap.Int("status_code", statusCode))

	response := CleanupResponse{
		Success:   false,
		Message:   message,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}
