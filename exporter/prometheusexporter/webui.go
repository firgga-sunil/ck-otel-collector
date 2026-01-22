// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter"

import (
	"embed"
	"net/http"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
)

//go:embed static/*
var staticFiles embed.FS

// WebUI provides HTTP endpoints for the metrics visualization UI
type WebUI struct {
	logger *zap.Logger
}

// NewWebUI creates a new web UI instance
func NewWebUI(logger *zap.Logger) *WebUI {
	return &WebUI{
		logger: logger,
	}
}

// IndexHandler serves the main UI page
func (ui *WebUI) IndexHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Serve the main HTML page
	indexHTML := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>OpenTelemetry Metrics Dashboard</title>
    <link rel="stylesheet" href="/static/style.css">
    <link href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.0.0/css/all.min.css" rel="stylesheet">
</head>
<body>
    <div class="container">
        <header class="header">
            <h1><i class="fas fa-chart-line"></i> OpenTelemetry Metrics Dashboard</h1>
            <div class="header-actions">
                <button id="refreshBtn" class="btn btn-primary">
                    <i class="fas fa-sync-alt"></i> Refresh
                </button>
                <span id="lastUpdate" class="last-update"></span>
            </div>
        </header>

        <div class="metrics-summary">
            <div class="summary-card">
                <h3>Total Metrics</h3>
                <span id="totalMetrics" class="summary-number">0</span>
            </div>
            <div class="summary-card">
                <h3>Services</h3>
                <span id="totalServices" class="summary-number">0</span>
            </div>
            <div class="summary-card">
                <h3>Active Errors</h3>
                <span id="totalErrors" class="summary-number">0</span>
            </div>
        </div>

        <div class="filters">
            <input type="text" id="searchInput" placeholder="Search metrics by name, flow_id, path_key, error_code..." class="search-input">
            <select id="serviceFilter" class="filter-select">
                <option value="">All Services</option>
            </select>
            <select id="typeFilter" class="filter-select">
                <option value="">All Types</option>
            </select>
            <button id="commonLabelsBtn" class="btn btn-info" title="Show/Hide Common Labels">
                <i class="fas fa-info-circle"></i> Common Labels
            </button>
        </div>

        <div id="commonLabelsPanel" class="common-labels-panel hidden">
            <h3><i class="fas fa-tags"></i> Common Labels</h3>
            <div id="commonLabelsContent" class="common-labels-content">
                <!-- Common labels will be displayed here -->
            </div>
        </div>

        <div id="loading" class="loading">
            <i class="fas fa-spinner fa-spin"></i> Loading metrics...
        </div>

        <div id="error" class="error" style="display: none;">
            <i class="fas fa-exclamation-triangle"></i>
            <span id="errorMessage"></span>
        </div>

        <div id="metricsContainer" class="metrics-container">
            <!-- Metrics will be dynamically loaded here -->
        </div>
    </div>

    <script src="/static/app.js"></script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(indexHTML))
}

// StaticHandler serves static files (CSS, JS)
func (ui *WebUI) StaticHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract the file path from the URL
	path := strings.TrimPrefix(r.URL.Path, "/static/")

	// Determine content type based on file extension
	var contentType string
	switch filepath.Ext(path) {
	case ".css":
		contentType = "text/css"
	case ".js":
		contentType = "application/javascript"
	case ".png":
		contentType = "image/png"
	case ".jpg", ".jpeg":
		contentType = "image/jpeg"
	case ".svg":
		contentType = "image/svg+xml"
	default:
		contentType = "text/plain"
	}

	// Read the file from embedded filesystem
	content, err := staticFiles.ReadFile("static/" + path)
	if err != nil {
		ui.logger.Error("Failed to read static file", zap.String("path", path), zap.Error(err))
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(http.StatusOK)
	w.Write(content)
}
