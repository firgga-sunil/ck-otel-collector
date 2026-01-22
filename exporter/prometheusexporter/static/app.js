// Metrics Dashboard Application
class MetricsDashboard {
    constructor() {
        this.metrics = [];
        this.filteredMetrics = [];
        this.services = new Set();
        this.metricTypes = new Set();
        this.commonLabels = {};
        this.refreshInterval = null;
        this.autoRefreshEnabled = true;
        this.collapsedServices = new Set();
        
        // Key labels that should be prominently displayed
        this.keyLabels = ['flow_id', 'mpk', 'path_key', 'fc', 'error_code', 'ipk', 'tp'];
        
        // Common labels that should be hidden by default
        this.commonLabelKeys = [
            'otel_aggregation_level', 'otel_aggregation_type', 'otel_component', 
            'otel_scope_name', 'otel_scope_schema_url', 'otel_scope_version',
            'agent_version', 'ck_cluster_name', 'ck_component', 'ck_namespace'
        ];
        
        this.init();
    }

    init() {
        this.bindEventListeners();
        this.loadMetrics();
        this.startAutoRefresh();
    }

    bindEventListeners() {
        // Refresh button
        document.getElementById('refreshBtn').addEventListener('click', () => {
            this.loadMetrics();
        });

        // Search input
        document.getElementById('searchInput').addEventListener('input', (e) => {
            this.filterMetrics();
        });

        // Service filter
        document.getElementById('serviceFilter').addEventListener('change', (e) => {
            this.filterMetrics();
        });

        // Type filter
        document.getElementById('typeFilter').addEventListener('change', (e) => {
            this.filterMetrics();
        });

        // Common labels toggle
        document.getElementById('commonLabelsBtn').addEventListener('click', () => {
            this.toggleCommonLabels();
        });

        // Escape key to close panels
        document.addEventListener('keydown', (e) => {
            if (e.key === 'Escape') {
                this.hideCommonLabels();
            }
        });
    }

    async loadMetrics() {
        try {
            this.showLoading();
            
            const response = await fetch('/metrics');
            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }
            
            const metricsText = await response.text();
            this.metrics = this.parsePrometheusMetrics(metricsText);
            this.extractCommonLabels();
            this.updateFilters();
            this.filterMetrics();
            this.updateSummary();
            this.updateLastRefresh();
            
            this.hideLoading();
            
        } catch (error) {
            console.error('Error loading metrics:', error);
            this.showError(`Failed to load metrics: ${error.message}`);
        }
    }

    parsePrometheusMetrics(text) {
        const metrics = [];
        const lines = text.split('\n');
        let currentMetricName = '';
        let currentType = '';
        let currentHelp = '';

        for (const line of lines) {
            const trimmedLine = line.trim();
            
            if (trimmedLine === '' || trimmedLine.startsWith('#')) {
                if (trimmedLine.startsWith('# HELP ')) {
                    currentHelp = trimmedLine.substring(7);
                    currentMetricName = currentHelp.split(' ')[0];
                } else if (trimmedLine.startsWith('# TYPE ')) {
                    const parts = trimmedLine.substring(7).split(' ');
                    currentMetricName = parts[0];
                    currentType = parts[1];
                }
                continue;
            }

            const metric = this.parseMetricLine(trimmedLine, currentType, currentHelp);
            if (metric) {
                metrics.push(metric);
            }
        }

        return metrics;
    }

    parseMetricLine(line, type, help) {
        // Parse metric line format: metric_name{labels} value [timestamp]
        const match = line.match(/^([a-zA-Z_:][a-zA-Z0-9_:]*(?:\{.*?\})?)\s+([^\s]+)(?:\s+([^\s]+))?$/);
        
        if (!match) return null;

        const [, metricWithLabels, value, timestamp] = match;
        
        // Extract metric name and labels
        const labelsMatch = metricWithLabels.match(/^([^{]+)(?:\{(.*)\})?$/);
        if (!labelsMatch) return null;

        const [, metricName, labelsStr] = labelsMatch;
        const labels = this.parseLabels(labelsStr || '');

        return {
            name: metricName,
            value: parseFloat(value),
            timestamp: timestamp ? parseInt(timestamp) * 1000 : Date.now(), // Convert to milliseconds
            type: type || 'unknown',
            help: help || '',
            labels: labels,
            serviceName: labels.ck_service_name || 'unknown'
        };
    }

    parseLabels(labelsStr) {
        const labels = {};
        if (!labelsStr) return labels;

        // Parse labels: key="value",key2="value2"
        const labelMatches = labelsStr.match(/([a-zA-Z_][a-zA-Z0-9_]*)="([^"]*)"/g);
        if (labelMatches) {
            for (const labelMatch of labelMatches) {
                const [, key, value] = labelMatch.match(/([a-zA-Z_][a-zA-Z0-9_]*)="([^"]*)"/);
                labels[key] = value;
            }
        }

        return labels;
    }

    extractCommonLabels() {
        this.commonLabels = {};
        
        if (this.metrics.length === 0) return;

        // Find labels that are common across all metrics
        const firstMetric = this.metrics[0];
        const potentialCommonLabels = this.commonLabelKeys.filter(key => 
            firstMetric.labels.hasOwnProperty(key)
        );

        for (const key of potentialCommonLabels) {
            const firstValue = firstMetric.labels[key];
            const isCommon = this.metrics.every(metric => 
                metric.labels[key] === firstValue
            );
            
            if (isCommon) {
                this.commonLabels[key] = firstValue;
            }
        }

        this.updateCommonLabelsDisplay();
    }

    updateCommonLabelsDisplay() {
        const container = document.getElementById('commonLabelsContent');
        container.innerHTML = '';

        if (Object.keys(this.commonLabels).length === 0) {
            container.innerHTML = '<p style="color: #666; font-style: italic;">No common labels found</p>';
            return;
        }

        Object.entries(this.commonLabels).forEach(([key, value]) => {
            const labelDiv = document.createElement('div');
            labelDiv.className = 'common-label';
            labelDiv.innerHTML = `
                <span class="common-label-key">${key}:</span>
                <span class="common-label-value">${value}</span>
            `;
            container.appendChild(labelDiv);
        });
    }

    updateFilters() {
        // Update services
        this.services.clear();
        this.metricTypes.clear();
        
        for (const metric of this.metrics) {
            this.services.add(metric.serviceName);
            this.metricTypes.add(metric.type);
        }

        // Update service filter dropdown
        const serviceFilter = document.getElementById('serviceFilter');
        serviceFilter.innerHTML = '<option value="">All Services</option>';
        
        Array.from(this.services).sort().forEach(service => {
            const option = document.createElement('option');
            option.value = service;
            option.textContent = service;
            serviceFilter.appendChild(option);
        });

        // Update type filter dropdown
        const typeFilter = document.getElementById('typeFilter');
        typeFilter.innerHTML = '<option value="">All Types</option>';
        
        Array.from(this.metricTypes).sort().forEach(type => {
            const option = document.createElement('option');
            option.value = type;
            option.textContent = type.charAt(0).toUpperCase() + type.slice(1);
            typeFilter.appendChild(option);
        });
    }

    filterMetrics() {
        const searchTerm = document.getElementById('searchInput').value.toLowerCase();
        const serviceFilter = document.getElementById('serviceFilter').value;
        const typeFilter = document.getElementById('typeFilter').value;

        this.filteredMetrics = this.metrics.filter(metric => {
            const searchableText = [
                metric.name,
                metric.labels.flow_id || '',
                metric.labels.mpk || '',
                metric.labels.path_key || '',
                metric.labels.error_code || '',
                metric.labels.fc || '',
                metric.labels.ipk || '',
                metric.labels.tp || ''
            ].join(' ').toLowerCase();
            
            const matchesSearch = !searchTerm || searchableText.includes(searchTerm);
            const matchesService = !serviceFilter || metric.serviceName === serviceFilter;
            const matchesType = !typeFilter || metric.type === typeFilter;

            return matchesSearch && matchesService && matchesType;
        });

        this.renderMetrics();
    }

    renderMetrics() {
        const container = document.getElementById('metricsContainer');
        container.innerHTML = '';

        // Group metrics by service
        const serviceGroups = {};
        for (const metric of this.filteredMetrics) {
            if (!serviceGroups[metric.serviceName]) {
                serviceGroups[metric.serviceName] = [];
            }
            serviceGroups[metric.serviceName].push(metric);
        }

        // Render each service group
        Object.keys(serviceGroups).sort().forEach(serviceName => {
            const serviceMetrics = serviceGroups[serviceName];
            const serviceGroupElement = this.createServiceGroup(serviceName, serviceMetrics);
            container.appendChild(serviceGroupElement);
        });

        if (this.filteredMetrics.length === 0) {
            container.innerHTML = `
                <div class="loading">
                    <i class="fas fa-search"></i>
                    No metrics found matching your criteria.
                </div>
            `;
        }
    }

    createServiceGroup(serviceName, metrics) {
        const serviceGroup = document.createElement('div');
        serviceGroup.className = 'service-group';

        // Count metrics by type and errors
        const typeCounts = {};
        let errorCount = 0;
        
        metrics.forEach(metric => {
            typeCounts[metric.type] = (typeCounts[metric.type] || 0) + 1;
            if (metric.labels.error_code && metric.labels.error_code !== '') {
                errorCount++;
            }
        });

        const typeCountsText = Object.entries(typeCounts)
            .map(([type, count]) => `${count} ${type}${count > 1 ? 's' : ''}`)
            .join(', ');

        const isCollapsed = this.collapsedServices.has(serviceName);

        serviceGroup.innerHTML = `
            <div class="service-header">
                <div class="service-header-content">
                    <div class="service-title" onclick="dashboard.toggleService('${serviceName}')">
                        <i class="fas fa-server"></i>
                        ${serviceName}
                    </div>
                    <button class="btn btn-danger btn-small delete-service-btn" onclick="dashboard.deleteService('${serviceName}', event)" title="Delete all metrics for this service">
                        <i class="fas fa-trash"></i> Delete
                    </button>
                    <div class="service-stats" onclick="dashboard.toggleService('${serviceName}')">
                        <span>${metrics.length} metrics</span>
                        <span>${typeCountsText}</span>
                        ${errorCount > 0 ? `<span style="color: #ff6b6b;">${errorCount} errors</span>` : ''}
                    </div>
                </div>
                <div class="service-toggle ${isCollapsed ? 'collapsed' : ''}" onclick="dashboard.toggleService('${serviceName}')">
                    <i class="fas fa-chevron-down"></i>
                </div>
            </div>
            <div class="metrics-table-container collapsible ${isCollapsed ? 'collapsed' : ''}">
                ${this.createMetricsTable(metrics)}
            </div>
        `;

        return serviceGroup;
    }

    createMetricsTable(metrics) {
        return `
            <table class="metrics-table">
                <thead>
                    <tr>
                        <th>Metric Name</th>
                        <th>Type</th>
                        <th>Value</th>
                        <th>Key Details</th>
                        <th>Timestamp</th>
                        <th>Path Key</th>
                    </tr>
                </thead>
                <tbody>
                    ${metrics.map(metric => this.createMetricRow(metric)).join('')}
                </tbody>
            </table>
        `;
    }

    createMetricRow(metric) {
        const formattedValue = this.formatMetricValue(metric.value);
        const formattedTimestamp = this.formatTimestamp(metric.timestamp);
        
        // Extract key labels for display
        const keyDetails = this.keyLabels
            .filter(key => metric.labels[key] && metric.labels[key] !== '')
            .map(key => ({
                key,
                value: metric.labels[key],
                isError: key === 'error_code'
            }));

        const pathKey = metric.labels.path_key || '';

        return `
            <tr>
                <td class="metric-name">${metric.name}</td>
                <td><span class="metric-type ${metric.type}">${metric.type}</span></td>
                <td class="metric-value">${formattedValue}</td>
                <td class="key-labels">
                    ${keyDetails.map(detail => `
                        <div class="key-label">
                            <span class="key-label-key">${detail.key}:</span>
                            <span class="key-label-value ${detail.isError ? 'error-code' : detail.key === 'flow_id' ? 'flow-id' : ''}">${detail.value}</span>
                        </div>
                    `).join('')}
                </td>
                <td class="metric-timestamp">${formattedTimestamp}</td>
                <td class="path-key" title="${pathKey}">${pathKey}</td>
            </tr>
        `;
    }

    formatMetricValue(value) {
        if (value === 0) return '0';
        if (Math.abs(value) >= 1e9) return (value / 1e9).toFixed(2) + 'B';
        if (Math.abs(value) >= 1e6) return (value / 1e6).toFixed(2) + 'M';
        if (Math.abs(value) >= 1e3) return (value / 1e3).toFixed(2) + 'K';
        if (value % 1 === 0) return value.toString();
        return value.toFixed(3).replace(/\.?0+$/, '');
    }

    formatTimestamp(timestamp) {
        const date = new Date(timestamp);
        const now = new Date();
        const diffMs = now - date;
        const diffMins = Math.floor(diffMs / 60000);
        const diffHours = Math.floor(diffMins / 60);
        const diffDays = Math.floor(diffHours / 24);

        if (diffMins < 1) return 'just now';
        if (diffMins < 60) return `${diffMins}m ago`;
        if (diffHours < 24) return `${diffHours}h ago`;
        if (diffDays < 7) return `${diffDays}d ago`;
        
        return date.toLocaleDateString() + ' ' + date.toLocaleTimeString([], {hour: '2-digit', minute:'2-digit'});
    }

    updateSummary() {
        const errorCount = this.metrics.filter(m => 
            m.labels.error_code && m.labels.error_code !== ''
        ).length;

        document.getElementById('totalMetrics').textContent = this.metrics.length;
        document.getElementById('totalServices').textContent = this.services.size;
        document.getElementById('totalErrors').textContent = errorCount;
    }

    updateLastRefresh() {
        const now = new Date();
        document.getElementById('lastUpdate').textContent = 
            `Last updated: ${now.toLocaleTimeString()}`;
    }

    toggleCommonLabels() {
        const panel = document.getElementById('commonLabelsPanel');
        panel.classList.toggle('hidden');
    }

    hideCommonLabels() {
        const panel = document.getElementById('commonLabelsPanel');
        panel.classList.add('hidden');
    }

    toggleService(serviceName) {
        if (this.collapsedServices.has(serviceName)) {
            this.collapsedServices.delete(serviceName);
        } else {
            this.collapsedServices.add(serviceName);
        }
        this.renderMetrics(); // Re-render to apply collapse state
    }

    async deleteService(serviceName, event) {
        // Prevent event bubbling to avoid triggering toggle
        event.stopPropagation();
        
        // Show confirmation dialog
        const confirmed = confirm(`Are you sure you want to delete all metrics for service "${serviceName}"? This action cannot be undone.`);
        if (!confirmed) {
            return;
        }

        try {
            // Disable the delete button and show loading state
            const deleteBtn = event.target.closest('.delete-service-btn');
            deleteBtn.disabled = true;
            deleteBtn.innerHTML = '<i class="fas fa-spinner fa-spin"></i> Deleting...';

            const response = await fetch('/cleanup', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    type: 'labels',
                    filters: {
                        ck_service_name: serviceName
                    }
                })
            });

            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }

            // Show success message
            this.showSuccessMessage(`Successfully deleted all metrics for service "${serviceName}"`);
            
            // Refresh the metrics to reflect the changes
            setTimeout(() => {
                this.loadMetrics();
            }, 1000);

        } catch (error) {
            console.error('Error deleting service:', error);
            this.showError(`Failed to delete service "${serviceName}": ${error.message}`);
            
            // Re-enable the delete button
            const deleteBtn = event.target.closest('.delete-service-btn');
            deleteBtn.disabled = false;
            deleteBtn.innerHTML = '<i class="fas fa-trash"></i> Delete';
        }
    }

    showLoading() {
        document.getElementById('loading').style.display = 'block';
        document.getElementById('error').style.display = 'none';
        document.getElementById('metricsContainer').style.display = 'none';
    }

    hideLoading() {
        document.getElementById('loading').style.display = 'none';
        document.getElementById('metricsContainer').style.display = 'block';
    }

    showError(message) {
        document.getElementById('loading').style.display = 'none';
        document.getElementById('error').style.display = 'block';
        document.getElementById('errorMessage').textContent = message;
        document.getElementById('metricsContainer').style.display = 'none';
    }

    showSuccessMessage(message) {
        // Create or update success message element
        let successElement = document.getElementById('successMessage');
        if (!successElement) {
            successElement = document.createElement('div');
            successElement.id = 'successMessage';
            successElement.className = 'success-message';
            document.querySelector('.container').insertBefore(successElement, document.getElementById('metricsContainer'));
        }
        
        successElement.innerHTML = `
            <i class="fas fa-check-circle"></i>
            <span>${message}</span>
        `;
        successElement.style.display = 'block';
        
        // Hide after 5 seconds
        setTimeout(() => {
            successElement.style.display = 'none';
        }, 5000);
    }

    startAutoRefresh() {
        if (this.refreshInterval) {
            clearInterval(this.refreshInterval);
        }
        
        // Auto-refresh every 30 seconds
        this.refreshInterval = setInterval(() => {
            if (this.autoRefreshEnabled) {
                this.loadMetrics();
            }
        }, 30000);
    }
}

// Initialize the dashboard when DOM is loaded
document.addEventListener('DOMContentLoaded', () => {
    window.dashboard = new MetricsDashboard();
});

// Handle visibility change to pause/resume auto-refresh
document.addEventListener('visibilitychange', () => {
    if (window.dashboard) {
        window.dashboard.autoRefreshEnabled = !document.hidden;
    }
});