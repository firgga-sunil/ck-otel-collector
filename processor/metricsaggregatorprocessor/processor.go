// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricsaggregatorprocessor

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// metricsAggregatorProcessor implements cross-resource metric aggregation
type metricsAggregatorProcessor struct {
	config *Config
	logger *zap.Logger
}

// aggregationState holds the state for ongoing aggregations
type aggregationState struct {
	values []float64
	count  int64
}

// newMetricsAggregatorProcessor creates a new cross-resource aggregation processor
func newMetricsAggregatorProcessor(config *Config, logger *zap.Logger) *metricsAggregatorProcessor {
	return &metricsAggregatorProcessor{
		config: config,
		logger: logger,
	}
}

// processMetrics processes metrics through cross-resource aggregation rules
func (p *metricsAggregatorProcessor) processMetrics(ctx context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	// Process each aggregation rule sequentially
	for _, rule := range p.config.AggregationRules {
		if err := p.processAggregationRule(md, rule); err != nil {
			p.logger.Error("Failed to process aggregation rule",
				zap.String("rule", rule.OutputMetricName),
				zap.Error(err))
			continue
		}
	}

	return md, nil
}

// processAggregationRule processes a single aggregation rule
func (p *metricsAggregatorProcessor) processAggregationRule(md pmetric.Metrics, rule AggregationRule) error {
	// Step 1: Collect matching metrics
	matchingMetrics := p.collectMatchingMetrics(md, rule)
	if len(matchingMetrics) == 0 {
		return nil // No metrics to aggregate
	}

	// Step 2: Aggregate collected metrics and get grouped results using global config
	groupedResults := p.aggregateMetricsByResourceContext(matchingMetrics, rule)
	if len(groupedResults) == 0 {
		return nil // Nothing to aggregate
	}

	// Step 3: Create separate resources for each resource context
	for _, result := range groupedResults {
		aggregatedRM := md.ResourceMetrics().AppendEmpty()

		// Set resource attributes for this specific resource context
		for key, value := range result.ResourceAttrs {
			aggregatedRM.Resource().Attributes().PutStr(key, value)
		}

		// Apply global output resource attributes (these mark the resource as aggregated)
		for key, value := range p.config.OutputResourceAttributes {
			aggregatedRM.Resource().Attributes().PutStr(key, value)
		}

		// Add the aggregated metric to this resource
		sm := aggregatedRM.ScopeMetrics().AppendEmpty()
		sm.Scope().SetName("metricsaggregator")
		sm.Scope().SetVersion("1.0.0")
		result.Metric.CopyTo(sm.Metrics().AppendEmpty())
	}

	// Step 4: Remove original metrics if needed (skip aggregated resources)
	if !rule.PreserveOriginalMetrics {
		p.removeOriginalMetrics(md, rule)
	}

	return nil
}

// MetricWithResource holds a metric along with its resource attributes
type MetricWithResource struct {
	Metric        pmetric.Metric
	ResourceAttrs pcommon.Map
}

// collectMatchingMetrics finds all metrics that match the rule pattern
func (p *metricsAggregatorProcessor) collectMatchingMetrics(md pmetric.Metrics, rule AggregationRule) []MetricWithResource {
	var matchingMetrics []MetricWithResource

	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		resourceAttrs := rm.Resource().Attributes()
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				metric := sm.Metrics().At(k)
				if p.matchesPattern(metric.Name(), rule) {
					matchingMetrics = append(matchingMetrics, MetricWithResource{
						Metric:        metric,
						ResourceAttrs: resourceAttrs,
					})
				}
			}
		}
	}

	return matchingMetrics
}

// matchesPattern checks if a metric name matches the rule pattern
func (p *metricsAggregatorProcessor) matchesPattern(metricName string, rule AggregationRule) bool {
	switch rule.MatchType {
	case "strict", "":
		return metricName == rule.MetricPattern
	case "regex":
		matched, err := regexp.MatchString(rule.MetricPattern, metricName)
		if err != nil {
			p.logger.Error("Invalid regex pattern",
				zap.String("pattern", rule.MetricPattern),
				zap.Error(err))
			return false
		}
		return matched
	default:
		return false
	}
}

// ResourceContextResult represents an aggregated metric for a specific resource context
type ResourceContextResult struct {
	Metric        pmetric.Metric
	ResourceAttrs map[string]string
}

// aggregateMetricsByResourceContext groups metrics and creates separate results for each resource context
func (p *metricsAggregatorProcessor) aggregateMetricsByResourceContext(metrics []MetricWithResource, rule AggregationRule) []ResourceContextResult {
	// Group metrics by labels using global configuration
	groups := p.groupMetricsByLabels(metrics, p.config.GroupByLabels)

	var results []ResourceContextResult

	// Process each group separately to create individual resource contexts
	for groupKey, groupMetrics := range groups {
		// Create result metric for this group
		resultMetric := pmetric.NewMetric()
		resultMetric.SetName(p.sanitizeMetricName(rule.OutputMetricName))
		resultMetric.SetDescription(fmt.Sprintf("Aggregated metric using %s aggregation", rule.AggregationType))

		// Determine output type
		outputType := rule.OutputMetricType
		if outputType == "" {
			outputType = "gauge" // default
		}

		// Create the metric type
		switch outputType {
		case "gauge":
			resultMetric.SetEmptyGauge()
		case "sum":
			resultMetric.SetEmptySum()
			resultMetric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
			resultMetric.Sum().SetIsMonotonic(true)
		case "histogram":
			resultMetric.SetEmptyHistogram()
		}

		// Calculate aggregated value and timestamps
		aggregatedValue := p.calculateAggregatedValue(groupMetrics, rule.AggregationType)
		timestamp := p.getLatestTimestamp(groupMetrics)

		// Add single data point for this group
		switch outputType {
		case "gauge":
			dp := resultMetric.Gauge().DataPoints().AppendEmpty()
			dp.SetDoubleValue(aggregatedValue)
			dp.SetTimestamp(timestamp)
			p.setDataPointLabelsFromGroupKey(dp.Attributes(), groupKey, p.config.GroupByLabels, groupMetrics)
		case "sum":
			dp := resultMetric.Sum().DataPoints().AppendEmpty()
			dp.SetDoubleValue(aggregatedValue)
			dp.SetTimestamp(timestamp)
			// TODO : Is this needed ?
			dp.SetStartTimestamp(p.getEarliestTimestamp(groupMetrics)) // Set start timestamp for sum..
			p.setDataPointLabelsFromGroupKey(dp.Attributes(), groupKey, p.config.GroupByLabels, groupMetrics)
		case "histogram":
			dp := resultMetric.Histogram().DataPoints().AppendEmpty()
			dp.SetSum(aggregatedValue)
			dp.SetCount(uint64(len(groupMetrics)))
			dp.SetTimestamp(timestamp)
			p.setDataPointLabelsFromGroupKey(dp.Attributes(), groupKey, p.config.GroupByLabels, groupMetrics)
		}

		// Extract resource attributes for this group
		resourceAttrs := p.extractResourceAttrsFromGroup(groupKey, p.config.GroupByLabels, groupMetrics)

		results = append(results, ResourceContextResult{
			Metric:        resultMetric,
			ResourceAttrs: resourceAttrs,
		})
	}

	return results
}

// groupMetricsByLabels groups metrics by specified label keys
func (p *metricsAggregatorProcessor) groupMetricsByLabels(metrics []MetricWithResource, groupByLabels []string) map[string][]MetricWithResource {
	groups := make(map[string][]MetricWithResource)

	for _, metricWithResource := range metrics {
		// Group each data point separately instead of the entire metric
		p.groupDataPointsByLabels(metricWithResource.Metric, metricWithResource.ResourceAttrs, groupByLabels, groups)
	}

	return groups
}

// groupDataPointsByLabels groups data points within a metric by their labels
// TODO: MEMORY OPTIMIZATION NEEDED - This implementation creates a new metric clone for each datapoint
// which is memory intensive for metrics with many datapoints. Consider implementing one of these solutions:
// 1. Store datapoint indices with metric references (MetricWithDatapoint struct)
// 2. Use lightweight value cache (MetricValueWithContext struct)
// 3. Smart filtering during extraction (re-evaluate grouping)
// See discussion: https://github.com/your-repo/issues/XXX
func (p *metricsAggregatorProcessor) groupDataPointsByLabels(metric pmetric.Metric, resourceAttrs pcommon.Map, groupByLabels []string, groups map[string][]MetricWithResource) {
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		dataPoints := metric.Gauge().DataPoints()
		for i := 0; i < dataPoints.Len(); i++ {
			dp := dataPoints.At(i)
			groupKey := p.buildGroupKeyFromPresentAttributes(resourceAttrs, dp.Attributes(), groupByLabels)

			// TODO: MEMORY INEFFICIENT - Creating new metric for each datapoint
			// This ensures functional correctness but uses excessive memory
			newMetric := pmetric.NewMetric()
			metric.CopyTo(newMetric)
			newMetric.SetEmptyGauge()
			newDataPoint := newMetric.Gauge().DataPoints().AppendEmpty()
			dp.CopyTo(newDataPoint)

			groups[groupKey] = append(groups[groupKey], MetricWithResource{
				Metric:        newMetric,
				ResourceAttrs: resourceAttrs,
			})
		}
	case pmetric.MetricTypeSum:
		dataPoints := metric.Sum().DataPoints()
		for i := 0; i < dataPoints.Len(); i++ {
			dp := dataPoints.At(i)
			groupKey := p.buildGroupKeyFromPresentAttributes(resourceAttrs, dp.Attributes(), groupByLabels)

			// TODO: MEMORY INEFFICIENT - Creating new metric for each datapoint
			newMetric := pmetric.NewMetric()
			metric.CopyTo(newMetric)
			newMetric.SetEmptySum()
			newMetric.Sum().SetAggregationTemporality(metric.Sum().AggregationTemporality())
			newMetric.Sum().SetIsMonotonic(metric.Sum().IsMonotonic())
			newDataPoint := newMetric.Sum().DataPoints().AppendEmpty()
			dp.CopyTo(newDataPoint)

			groups[groupKey] = append(groups[groupKey], MetricWithResource{
				Metric:        newMetric,
				ResourceAttrs: resourceAttrs,
			})
		}
	case pmetric.MetricTypeHistogram:
		dataPoints := metric.Histogram().DataPoints()
		for i := 0; i < dataPoints.Len(); i++ {
			dp := dataPoints.At(i)
			groupKey := p.buildGroupKeyFromPresentAttributes(resourceAttrs, dp.Attributes(), groupByLabels)

			// TODO: MEMORY INEFFICIENT - Creating new metric for each datapoint
			newMetric := pmetric.NewMetric()
			metric.CopyTo(newMetric)
			newMetric.SetEmptyHistogram()
			newMetric.Histogram().SetAggregationTemporality(metric.Histogram().AggregationTemporality())
			newDataPoint := newMetric.Histogram().DataPoints().AppendEmpty()
			dp.CopyTo(newDataPoint)

			groups[groupKey] = append(groups[groupKey], MetricWithResource{
				Metric:        newMetric,
				ResourceAttrs: resourceAttrs,
			})
		}
	}
}

// buildGroupKeyFromPresentAttributes creates a group key from both resource and datapoint attributes
// Returns the group key constructed from present labels only
func (p *metricsAggregatorProcessor) buildGroupKeyFromPresentAttributes(resourceAttrs pcommon.Map, dataPointAttrs pcommon.Map, groupByLabels []string) string {
	if len(groupByLabels) == 0 {
		return "all" // Single group for all metrics
	}

	var keyParts []string

	for _, label := range groupByLabels {
		// Look in datapoint attributes first, then resource attributes
		var value string
		var found bool

		if val, exists := dataPointAttrs.Get(label); exists {
			value = val.AsString()
			found = true
		} else if val, exists := resourceAttrs.Get(label); exists {
			value = val.AsString()
			found = true
		}

		// Only include labels that are actually present (even if empty)
		if found {
			keyParts = append(keyParts, label+"="+value)
		}
		// Missing labels are completely excluded
	}

	// Build group key from present labels only
	if len(keyParts) == 0 {
		return "all" // default for no present labels
	}

	return strings.Join(keyParts, "|")
}

// getLatestTimestamp gets the latest timestamp from a group of metrics
func (p *metricsAggregatorProcessor) getLatestTimestamp(metrics []MetricWithResource) pcommon.Timestamp {
	var latestTimestamp pcommon.Timestamp = 0

	for _, metricWithResource := range metrics {
		metric := metricWithResource.Metric
		switch metric.Type() {
		case pmetric.MetricTypeGauge:
			dataPoints := metric.Gauge().DataPoints()
			for i := 0; i < dataPoints.Len(); i++ {
				ts := dataPoints.At(i).Timestamp()
				if ts > latestTimestamp {
					latestTimestamp = ts
				}
			}
		case pmetric.MetricTypeSum:
			dataPoints := metric.Sum().DataPoints()
			for i := 0; i < dataPoints.Len(); i++ {
				ts := dataPoints.At(i).Timestamp()
				if ts > latestTimestamp {
					latestTimestamp = ts
				}
			}
		case pmetric.MetricTypeHistogram:
			dataPoints := metric.Histogram().DataPoints()
			for i := 0; i < dataPoints.Len(); i++ {
				ts := dataPoints.At(i).Timestamp()
				if ts > latestTimestamp {
					latestTimestamp = ts
				}
			}
		}
	}

	// If no timestamp found, use current time
	if latestTimestamp == 0 {
		latestTimestamp = pcommon.NewTimestampFromTime(time.Now())
	}

	return latestTimestamp
}

// getEarliestTimestamp gets the earliest timestamp from a group of metrics
func (p *metricsAggregatorProcessor) getEarliestTimestamp(metrics []MetricWithResource) pcommon.Timestamp {
	var earliestTimestamp pcommon.Timestamp = pcommon.Timestamp(^uint64(0)) // Max value

	for _, metricWithResource := range metrics {
		metric := metricWithResource.Metric
		switch metric.Type() {
		case pmetric.MetricTypeGauge:
			dataPoints := metric.Gauge().DataPoints()
			for i := 0; i < dataPoints.Len(); i++ {
				ts := dataPoints.At(i).Timestamp()
				if ts < earliestTimestamp && ts > 0 {
					earliestTimestamp = ts
				}
			}
		case pmetric.MetricTypeSum:
			dataPoints := metric.Sum().DataPoints()
			for i := 0; i < dataPoints.Len(); i++ {
				startTs := dataPoints.At(i).StartTimestamp()
				if startTs < earliestTimestamp && startTs > 0 {
					earliestTimestamp = startTs
				}
			}
		case pmetric.MetricTypeHistogram:
			dataPoints := metric.Histogram().DataPoints()
			for i := 0; i < dataPoints.Len(); i++ {
				startTs := dataPoints.At(i).StartTimestamp()
				if startTs < earliestTimestamp && startTs > 0 {
					earliestTimestamp = startTs
				}
			}
		}
	}

	// If no timestamp found, use current time minus 1 minute
	if earliestTimestamp == pcommon.Timestamp(^uint64(0)) {
		earliestTimestamp = pcommon.NewTimestampFromTime(time.Now().Add(-time.Minute))
	}

	return earliestTimestamp
}

// sanitizeMetricName ensures the metric name is valid for Prometheus
func (p *metricsAggregatorProcessor) sanitizeMetricName(name string) string {
	// Prometheus metric names must match [a-zA-Z_:][a-zA-Z0-9_:]*
	// Replace invalid characters with underscores
	sanitized := regexp.MustCompile(`[^a-zA-Z0-9_:]`).ReplaceAllString(name, "_")

	// Ensure it starts with a valid character
	if len(sanitized) > 0 && !regexp.MustCompile(`^[a-zA-Z_:]`).MatchString(sanitized[:1]) {
		sanitized = "_" + sanitized
	}

	return sanitized
}

// setLabelsFromGroupKey sets labels on attributes from group key
func (p *metricsAggregatorProcessor) setLabelsFromGroupKey(attributes pcommon.Map, groupKey string, groupByLabels []string) {
	if groupKey == "all" || len(groupByLabels) == 0 {
		return
	}

	// Parse group key back into labels
	// Format: "label1=value1|label2=value2"
	parts := regexp.MustCompile(`\|`).Split(groupKey, -1)

	for _, part := range parts {
		if keyValue := regexp.MustCompile(`=`).Split(part, 2); len(keyValue) == 2 {
			attributes.PutStr(keyValue[0], keyValue[1])
		}
	}
}

// calculateAggregatedValue calculates the aggregated value from multiple metrics
func (p *metricsAggregatorProcessor) calculateAggregatedValue(metrics []MetricWithResource, aggregationType string) float64 {
	var values []float64

	// Extract values from all metrics
	for _, metricWithResource := range metrics {
		metricValues := p.extractValuesFromMetric(metricWithResource.Metric)
		values = append(values, metricValues...)
	}

	if len(values) == 0 {
		return 0
	}

	// Calculate based on aggregation type
	switch aggregationType {
	case "sum", "":
		sum := 0.0
		for _, v := range values {
			sum += v
		}
		return sum
	case "mean":
		sum := 0.0
		for _, v := range values {
			sum += v
		}
		return sum / float64(len(values))
	case "min":
		min := values[0]
		for _, v := range values[1:] {
			if v < min {
				min = v
			}
		}
		return min
	case "max":
		max := values[0]
		for _, v := range values[1:] {
			if v > max {
				max = v
			}
		}
		return max
	case "count":
		return float64(len(values))
	default:
		return 0
	}
}

// extractValuesFromMetric extracts numeric values from a metric
func (p *metricsAggregatorProcessor) extractValuesFromMetric(metric pmetric.Metric) []float64 {
	var values []float64

	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		for i := 0; i < metric.Gauge().DataPoints().Len(); i++ {
			dp := metric.Gauge().DataPoints().At(i)
			switch dp.ValueType() {
			case pmetric.NumberDataPointValueTypeDouble:
				values = append(values, dp.DoubleValue())
			case pmetric.NumberDataPointValueTypeInt:
				values = append(values, float64(dp.IntValue()))
			}
		}
	case pmetric.MetricTypeSum:
		for i := 0; i < metric.Sum().DataPoints().Len(); i++ {
			dp := metric.Sum().DataPoints().At(i)
			switch dp.ValueType() {
			case pmetric.NumberDataPointValueTypeDouble:
				values = append(values, dp.DoubleValue())
			case pmetric.NumberDataPointValueTypeInt:
				values = append(values, float64(dp.IntValue()))
			}
		}
	case pmetric.MetricTypeHistogram:
		for i := 0; i < metric.Histogram().DataPoints().Len(); i++ {
			dp := metric.Histogram().DataPoints().At(i)
			values = append(values, dp.Sum())
		}
	}

	return values
}

// removeOriginalMetrics removes original metrics while preserving aggregated ones
// Uses resource attributes to distinguish between original and aggregated resources
func (p *metricsAggregatorProcessor) removeOriginalMetrics(md pmetric.Metrics, rule AggregationRule) {
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)

		// Check if this resource has aggregated marker attributes using global config
		isAggregatedResource := p.hasAggregatedMarkerAttributes(rm.Resource().Attributes(), p.config.OutputResourceAttributes)

		// Skip removal for aggregated resources (optimization)
		if isAggregatedResource {
			continue
		}

		// This is an original resource - remove matching metrics from all scopes
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)

			// Remove metrics that match the pattern
			// RemoveIf handles internal iteration and removal safely
			sm.Metrics().RemoveIf(func(metric pmetric.Metric) bool {
				return p.matchesPattern(metric.Name(), rule)
			})
		}
	}
}

// hasAggregatedMarkerAttributes checks if a resource has the marker attributes that identify it as aggregated
func (p *metricsAggregatorProcessor) hasAggregatedMarkerAttributes(resourceAttrs pcommon.Map, markerAttrs map[string]string) bool {
	// Check if all marker attributes are present with correct values
	for key, expectedValue := range markerAttrs {
		if actualValue, exists := resourceAttrs.Get(key); !exists || actualValue.AsString() != expectedValue {
			return false
		}
	}
	return true
}

// extractResourceAttrsFromGroup extracts resource attributes for a specific group
// Only extracts attributes that were actually present in the input data
func (p *metricsAggregatorProcessor) extractResourceAttrsFromGroup(groupKey string, groupByLabels []string, metrics []MetricWithResource) map[string]string {
	resourceAttrs := make(map[string]string)

	if groupKey == "all" || len(groupByLabels) == 0 || len(metrics) == 0 {
		return resourceAttrs
	}

	// Get the first metric's resource attributes as a reference
	firstMetricResourceAttrs := metrics[0].ResourceAttrs

	// Parse group key back into labels
	// Format: "label1=value1|label2=value2"
	parts := regexp.MustCompile(`\|`).Split(groupKey, -1)

	for _, part := range parts {
		if keyValue := regexp.MustCompile(`=`).Split(part, 2); len(keyValue) == 2 {
			labelName := keyValue[0]
			labelValue := keyValue[1]

			// Only set as resource attribute if it exists in the original resource attributes
			// This ensures we only promote actual resource-level attributes, not datapoint attributes
			if _, exists := firstMetricResourceAttrs.Get(labelName); exists {
				resourceAttrs[labelName] = labelValue
			}
		}
	}

	return resourceAttrs
}

// setDataPointLabelsFromGroupKey sets labels on attributes from group key
// Only sets labels that were actually present in the input data
func (p *metricsAggregatorProcessor) setDataPointLabelsFromGroupKey(attributes pcommon.Map, groupKey string, groupByLabels []string, metrics []MetricWithResource) {
	if groupKey == "all" || len(groupByLabels) == 0 || len(metrics) == 0 {
		return
	}

	// Get the first metric to determine which attributes are resource-level vs datapoint-level
	firstMetric := metrics[0]
	resourceAttrs := firstMetric.ResourceAttrs

	// Parse group key back into labels
	// Format: "label1=value1|label2=value2"
	parts := regexp.MustCompile(`\|`).Split(groupKey, -1)

	for _, part := range parts {
		if keyValue := regexp.MustCompile(`=`).Split(part, 2); len(keyValue) == 2 {
			labelKey := keyValue[0]
			labelValue := keyValue[1]

			// Only set this attribute if it's NOT a resource-level attribute
			// This ensures we only set datapoint-level attributes
			if _, isResourceAttr := resourceAttrs.Get(labelKey); !isResourceAttr {
				attributes.PutStr(labelKey, labelValue)
			}
		}
	}
}
