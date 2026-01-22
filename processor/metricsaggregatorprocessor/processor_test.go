// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricsaggregatorprocessor

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func TestMetricsAggregatorProcessor_ProcessMetrics(t *testing.T) {
	tests := []struct {
		name          string
		config        *Config
		inputMetrics  pmetric.Metrics
		expectedCount int
		expectedNames []string
	}{
		{
			name: "sum aggregation",
			config: &Config{
				GroupByLabels: []string{},
				OutputResourceAttributes: map[string]string{
					"aggregation.level": "cluster",
				},
				AggregationRules: []AggregationRule{
					{
						MetricPattern:           "test_metric",
						MatchType:               "strict",
						OutputMetricName:        "aggregated_test_metric",
						AggregationType:         "sum",
						PreserveOriginalMetrics: false,
					},
				},
			},
			inputMetrics:  generateTestMetrics([]string{"test_metric", "test_metric", "other_metric"}, []float64{10, 20, 5}),
			expectedCount: 2, // aggregated metric + other_metric
			expectedNames: []string{"aggregated_test_metric", "other_metric"},
		},
		{
			name: "regex aggregation with mean",
			config: &Config{
				GroupByLabels: []string{"service"},
				OutputResourceAttributes: map[string]string{
					"aggregation.level": "cluster",
				},
				AggregationRules: []AggregationRule{
					{
						MetricPattern:           ".*_metric$",
						MatchType:               "regex",
						OutputMetricName:        "mean_aggregated_metric",
						AggregationType:         "mean",
						PreserveOriginalMetrics: false,
					},
				},
			},
			inputMetrics:  generateTestMetricsWithLabels(),
			expectedCount: 2, // two aggregated metrics (one per service group)
			expectedNames: []string{"mean_aggregated_metric", "mean_aggregated_metric"},
		},
		{
			name: "preserve original metrics",
			config: &Config{
				GroupByLabels: []string{},
				OutputResourceAttributes: map[string]string{
					"aggregation.level": "cluster",
				},
				AggregationRules: []AggregationRule{
					{
						MetricPattern:           "test_metric",
						MatchType:               "strict",
						OutputMetricName:        "aggregated_test_metric",
						AggregationType:         "sum",
						PreserveOriginalMetrics: true,
					},
				},
			},
			inputMetrics:  generateTestMetrics([]string{"test_metric", "test_metric"}, []float64{10, 20}),
			expectedCount: 3, // 2 original + 1 aggregated
			expectedNames: []string{"test_metric", "test_metric", "aggregated_test_metric"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := newMetricsAggregatorProcessor(tt.config, zap.NewNop())

			result, err := processor.processMetrics(context.Background(), tt.inputMetrics)
			require.NoError(t, err)

			// Count total metrics
			totalMetrics := 0
			var actualNames []string

			for i := 0; i < result.ResourceMetrics().Len(); i++ {
				rm := result.ResourceMetrics().At(i)
				for j := 0; j < rm.ScopeMetrics().Len(); j++ {
					sm := rm.ScopeMetrics().At(j)
					for k := 0; k < sm.Metrics().Len(); k++ {
						metric := sm.Metrics().At(k)
						actualNames = append(actualNames, metric.Name())
						totalMetrics++
					}
				}
			}

			assert.Equal(t, tt.expectedCount, totalMetrics)
			// Note: Order might vary, so we check if expected names are present
			for _, expectedName := range tt.expectedNames {
				assert.Contains(t, actualNames, expectedName)
			}
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectedErr string
	}{
		{
			name: "valid config",
			config: &Config{
				GroupByLabels: []string{"service"},
				OutputResourceAttributes: map[string]string{
					"otel_output_metric": "true",
				},
				AggregationRules: []AggregationRule{
					{
						MetricPattern:    "test_metric",
						OutputMetricName: "aggregated_metric",
						AggregationType:  "sum",
					},
				},
			},
			expectedErr: "",
		},
		{
			name: "missing group_by_labels",
			config: &Config{
				OutputResourceAttributes: map[string]string{
					"otel_output_metric": "true",
				},
				AggregationRules: []AggregationRule{
					{
						MetricPattern:    "test_metric",
						OutputMetricName: "aggregated_metric",
					},
				},
			},
			expectedErr: "group_by_labels cannot be empty",
		},
		{
			name: "missing output_resource_attributes",
			config: &Config{
				GroupByLabels: []string{"service"},
				AggregationRules: []AggregationRule{
					{
						MetricPattern:    "test_metric",
						OutputMetricName: "aggregated_metric",
					},
				},
			},
			expectedErr: "output_resource_attributes cannot be empty",
		},
		{
			name: "missing aggregation rules",
			config: &Config{
				GroupByLabels: []string{"service"},
				OutputResourceAttributes: map[string]string{
					"otel_output_metric": "true",
				},
				AggregationRules: []AggregationRule{},
			},
			expectedErr: "at least one aggregation rule must be specified",
		},
		{
			name: "invalid aggregation rule - missing metric pattern",
			config: &Config{
				GroupByLabels: []string{"service"},
				OutputResourceAttributes: map[string]string{
					"otel_output_metric": "true",
				},
				AggregationRules: []AggregationRule{
					{
						OutputMetricName: "aggregated_metric",
					},
				},
			},
			expectedErr: "metric_pattern cannot be empty",
		},
		{
			name: "invalid aggregation rule - missing output metric name",
			config: &Config{
				GroupByLabels: []string{"service"},
				OutputResourceAttributes: map[string]string{
					"otel_output_metric": "true",
				},
				AggregationRules: []AggregationRule{
					{
						MetricPattern: "test_metric",
					},
				},
			},
			expectedErr: "output_metric_name cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			}
		})
	}
}

// Helper functions for testing
func generateTestMetrics(names []string, values []float64) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()

	for i, name := range names {
		metric := sm.Metrics().AppendEmpty()
		metric.SetName(name)

		gauge := metric.SetEmptyGauge()
		dp := gauge.DataPoints().AppendEmpty()
		if i < len(values) {
			dp.SetDoubleValue(values[i])
		} else {
			dp.SetDoubleValue(0)
		}
	}

	return md
}

func generateTestMetricsWithLabels() pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()

	// Create metrics with different labels
	names := []string{"test_metric", "another_metric", "third_metric"}
	values := []float64{10, 20, 30}
	services := []string{"service-a", "service-b", "service-a"}

	for i, name := range names {
		metric := sm.Metrics().AppendEmpty()
		metric.SetName(name)

		gauge := metric.SetEmptyGauge()
		dp := gauge.DataPoints().AppendEmpty()
		dp.SetDoubleValue(values[i])
		dp.Attributes().PutStr("service", services[i])
	}

	return md
}

func TestCrossResourceProcessor_BasicAggregation(t *testing.T) {
	cfg := &Config{
		GroupByLabels: []string{"agent_version"},
		OutputResourceAttributes: map[string]string{
			"aggregation.level": "cluster",
		},
		AggregationRules: []AggregationRule{
			{
				MetricPattern:           "throughput",
				MatchType:               "strict",
				OutputMetricName:        "cluster_throughput",
				AggregationType:         "sum",
				PreserveOriginalMetrics: false,
			},
		},
	}

	// Create processor
	processor, err := createTestProcessor(cfg)
	require.NoError(t, err)

	// Create test metrics
	md := createTestMetrics()

	// Process metrics
	result, err := processor.processMetrics(context.Background(), md)
	require.NoError(t, err)

	// Verify results
	assert.Greater(t, result.ResourceMetrics().Len(), 0)

	// Find the aggregated metric
	found := false
	rms := result.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				metric := sm.Metrics().At(k)
				if metric.Name() == "cluster_throughput" {
					found = true
					// Check the actual type and handle accordingly
					switch metric.Type() {
					case pmetric.MetricTypeSum:
						assert.Greater(t, metric.Sum().DataPoints().Len(), 0)
					case pmetric.MetricTypeGauge:
						assert.Greater(t, metric.Gauge().DataPoints().Len(), 0)
					default:
						t.Errorf("Unexpected metric type: %v", metric.Type())
					}
				}
			}
		}
	}
	assert.True(t, found, "Aggregated metric not found")
}

func TestCrossResourceProcessor_RegexMatching(t *testing.T) {
	// Create processor config with regex
	cfg := &Config{
		GroupByLabels: []string{},
		OutputResourceAttributes: map[string]string{
			"aggregation.level": "cluster",
		},
		AggregationRules: []AggregationRule{
			{
				MetricPattern:           ".*_latency",
				MatchType:               "regex",
				OutputMetricName:        "cluster_latency_total",
				AggregationType:         "mean",
				PreserveOriginalMetrics: true,
			},
		},
	}

	// Create processor
	processor, err := createTestProcessor(cfg)
	require.NoError(t, err)

	// Create test metrics with latency metrics
	md := createTestMetricsWithLatency()

	// Process metrics
	result, err := processor.processMetrics(context.Background(), md)
	require.NoError(t, err)

	// Verify aggregated metric exists
	found := false
	rms := result.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				metric := sm.Metrics().At(k)
				if metric.Name() == "cluster_latency_total" {
					found = true
				}
			}
		}
	}
	assert.True(t, found, "Regex-matched aggregated metric not found")
}

func TestCrossResourceProcessor_MultipleRules(t *testing.T) {
	// Create processor config with multiple rules
	cfg := &Config{
		GroupByLabels: []string{},
		OutputResourceAttributes: map[string]string{
			"aggregation.level": "cluster",
		},
		AggregationRules: []AggregationRule{
			{
				MetricPattern:           "throughput",
				MatchType:               "strict",
				OutputMetricName:        "cluster_throughput",
				AggregationType:         "sum",
				PreserveOriginalMetrics: false,
			},
			{
				MetricPattern:           "response_time",
				MatchType:               "strict",
				OutputMetricName:        "cluster_avg_response_time",
				AggregationType:         "mean",
				PreserveOriginalMetrics: false,
			},
		},
	}

	// Create processor
	processor, err := createTestProcessor(cfg)
	require.NoError(t, err)

	// Create test metrics
	md := createTestMetricsWithMultipleTypes()

	// Process metrics
	result, err := processor.processMetrics(context.Background(), md)
	require.NoError(t, err)

	// Verify both aggregated metrics exist
	foundThroughput := false
	foundResponseTime := false

	rms := result.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				metric := sm.Metrics().At(k)
				if metric.Name() == "cluster_throughput" {
					foundThroughput = true
				}
				if metric.Name() == "cluster_avg_response_time" {
					foundResponseTime = true
				}
			}
		}
	}

	assert.True(t, foundThroughput, "Throughput aggregated metric not found")
	assert.True(t, foundResponseTime, "Response time aggregated metric not found")
}

func TestCrossResourceProcessor_NoMatches(t *testing.T) {
	// Create processor config that won't match anything
	cfg := &Config{
		GroupByLabels: []string{},
		OutputResourceAttributes: map[string]string{
			"aggregation.level": "cluster",
		},
		AggregationRules: []AggregationRule{
			{
				MetricPattern:           "nonexistent_metric",
				MatchType:               "strict",
				OutputMetricName:        "cluster_nonexistent",
				AggregationType:         "sum",
				PreserveOriginalMetrics: false,
			},
		},
	}

	// Create processor
	processor, err := createTestProcessor(cfg)
	require.NoError(t, err)

	// Create test metrics
	md := createTestMetrics()
	originalCount := countMetrics(md)

	// Process metrics
	result, err := processor.processMetrics(context.Background(), md)
	require.NoError(t, err)

	// Should have same number of metrics (no aggregation occurred)
	resultCount := countMetrics(result)
	assert.Equal(t, originalCount, resultCount)
}

func TestMetricsAggregatorProcessor_MultiplePathKeys(t *testing.T) {
	config := &Config{
		GroupByLabels: []string{"path_key"},
		OutputResourceAttributes: map[string]string{
			"aggregated": "true",
		},
		AggregationRules: []AggregationRule{
			{
				MetricPattern:           "throughput",
				MatchType:               "strict",
				OutputMetricName:        "cluster_throughput",
				AggregationType:         "sum",
				PreserveOriginalMetrics: false,
			},
		},
	}

	processor := newMetricsAggregatorProcessor(config, zap.NewNop())

	// Create test metrics with different pathKeys
	md := pmetric.NewMetrics()

	// Create multiple resources (simulating different pods)
	for i := 0; i < 3; i++ {
		rm := md.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().PutStr("pod_name", fmt.Sprintf("pod-%d", i))

		sm := rm.ScopeMetrics().AppendEmpty()

		// Create throughput metrics with different pathKeys
		for j := 0; j < 3; j++ {
			metric := sm.Metrics().AppendEmpty()
			metric.SetName("throughput")

			gauge := metric.SetEmptyGauge()
			dp := gauge.DataPoints().AppendEmpty()
			dp.SetDoubleValue(float64(10 + i + j)) // Different values
			dp.Attributes().PutStr("path_key", fmt.Sprintf("/api/v%d", j+1))
		}
	}

	result, err := processor.processMetrics(context.Background(), md)
	require.NoError(t, err)

	// Find all aggregated metrics (should be 3 separate resources, one per pathKey)
	var aggregatedMetrics []pmetric.Metric

	for i := 0; i < result.ResourceMetrics().Len(); i++ {
		rm := result.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				metric := sm.Metrics().At(k)
				if metric.Name() == "cluster_throughput" {
					aggregatedMetrics = append(aggregatedMetrics, metric)
				}
			}
		}
	}

	require.Equal(t, 3, len(aggregatedMetrics), "Should have 3 aggregated metrics (one per pathKey group)")

	// Collect all pathKeys and values from all metrics
	pathKeysFound := make(map[string]float64)
	for _, metric := range aggregatedMetrics {
		require.Equal(t, pmetric.MetricTypeGauge, metric.Type())
		dataPoints := metric.Gauge().DataPoints()
		require.Equal(t, 1, dataPoints.Len(), "Each metric should have exactly 1 data point")

		dp := dataPoints.At(0)
		pathKey, exists := dp.Attributes().Get("path_key")
		require.True(t, exists, "path_key attribute should exist")
		pathKeysFound[pathKey.AsString()] = dp.DoubleValue()
	}

	// Should have all 3 pathKeys
	assert.Contains(t, pathKeysFound, "/api/v1")
	assert.Contains(t, pathKeysFound, "/api/v2")
	assert.Contains(t, pathKeysFound, "/api/v3")

	// Verify aggregated values (sum across all pods for each pathKey)
	// For /api/v1: 10 + 11 + 12 = 33
	// For /api/v2: 11 + 12 + 13 = 36
	// For /api/v3: 12 + 13 + 14 = 39
	assert.Equal(t, 33.0, pathKeysFound["/api/v1"])
	assert.Equal(t, 36.0, pathKeysFound["/api/v2"])
	assert.Equal(t, 39.0, pathKeysFound["/api/v3"])
}

// Helper functions

func createTestProcessor(cfg *Config) (*metricsAggregatorProcessor, error) {
	return newMetricsAggregatorProcessor(cfg, zap.NewNop()), nil
}

func createTestMetrics() pmetric.Metrics {
	md := pmetric.NewMetrics()

	// Resource 1
	rm1 := md.ResourceMetrics().AppendEmpty()
	rm1.Resource().Attributes().PutStr("service.name", "service1")
	rm1.Resource().Attributes().PutStr("node.id", "node1")

	sm1 := rm1.ScopeMetrics().AppendEmpty()
	sm1.Scope().SetName("test-scope")

	// Throughput metric
	metric1 := sm1.Metrics().AppendEmpty()
	metric1.SetName("throughput")
	metric1.SetUnit("req/s")
	metric1.SetEmptySum()
	dp1 := metric1.Sum().DataPoints().AppendEmpty()
	dp1.SetDoubleValue(100.0)
	dp1.Attributes().PutStr("agent_version", "1.0")
	dp1.SetTimestamp(pcommon.NewTimestampFromTime(testTime))

	// Resource 2
	rm2 := md.ResourceMetrics().AppendEmpty()
	rm2.Resource().Attributes().PutStr("service.name", "service2")
	rm2.Resource().Attributes().PutStr("node.id", "node2")

	sm2 := rm2.ScopeMetrics().AppendEmpty()
	sm2.Scope().SetName("test-scope")

	// Throughput metric
	metric2 := sm2.Metrics().AppendEmpty()
	metric2.SetName("throughput")
	metric2.SetUnit("req/s")
	metric2.SetEmptySum()
	dp2 := metric2.Sum().DataPoints().AppendEmpty()
	dp2.SetDoubleValue(150.0)
	dp2.Attributes().PutStr("agent_version", "1.0")
	dp2.SetTimestamp(pcommon.NewTimestampFromTime(testTime))

	return md
}

func createTestMetricsWithLatency() pmetric.Metrics {
	md := pmetric.NewMetrics()

	// Resource 1
	rm1 := md.ResourceMetrics().AppendEmpty()
	rm1.Resource().Attributes().PutStr("service.name", "service1")

	sm1 := rm1.ScopeMetrics().AppendEmpty()
	sm1.Scope().SetName("test-scope")

	// API latency metric
	metric1 := sm1.Metrics().AppendEmpty()
	metric1.SetName("api_latency")
	metric1.SetUnit("ms")
	metric1.SetEmptyGauge()
	dp1 := metric1.Gauge().DataPoints().AppendEmpty()
	dp1.SetDoubleValue(50.0)
	dp1.SetTimestamp(pcommon.NewTimestampFromTime(testTime))

	// DB latency metric
	metric2 := sm1.Metrics().AppendEmpty()
	metric2.SetName("db_latency")
	metric2.SetUnit("ms")
	metric2.SetEmptyGauge()
	dp2 := metric2.Gauge().DataPoints().AppendEmpty()
	dp2.SetDoubleValue(30.0)
	dp2.SetTimestamp(pcommon.NewTimestampFromTime(testTime))

	return md
}

func createTestMetricsWithMultipleTypes() pmetric.Metrics {
	md := pmetric.NewMetrics()

	// Resource 1
	rm1 := md.ResourceMetrics().AppendEmpty()
	rm1.Resource().Attributes().PutStr("service.name", "service1")

	sm1 := rm1.ScopeMetrics().AppendEmpty()
	sm1.Scope().SetName("test-scope")

	// Throughput metric
	metric1 := sm1.Metrics().AppendEmpty()
	metric1.SetName("throughput")
	metric1.SetUnit("req/s")
	metric1.SetEmptySum()
	dp1 := metric1.Sum().DataPoints().AppendEmpty()
	dp1.SetDoubleValue(100.0)
	dp1.SetTimestamp(pcommon.NewTimestampFromTime(testTime))

	// Response time metric
	metric2 := sm1.Metrics().AppendEmpty()
	metric2.SetName("response_time")
	metric2.SetUnit("ms")
	metric2.SetEmptyGauge()
	dp2 := metric2.Gauge().DataPoints().AppendEmpty()
	dp2.SetDoubleValue(250.0)
	dp2.SetTimestamp(pcommon.NewTimestampFromTime(testTime))

	// Resource 2
	rm2 := md.ResourceMetrics().AppendEmpty()
	rm2.Resource().Attributes().PutStr("service.name", "service2")

	sm2 := rm2.ScopeMetrics().AppendEmpty()
	sm2.Scope().SetName("test-scope")

	// Throughput metric
	metric3 := sm2.Metrics().AppendEmpty()
	metric3.SetName("throughput")
	metric3.SetUnit("req/s")
	metric3.SetEmptySum()
	dp3 := metric3.Sum().DataPoints().AppendEmpty()
	dp3.SetDoubleValue(200.0)
	dp3.SetTimestamp(pcommon.NewTimestampFromTime(testTime))

	// Response time metric
	metric4 := sm2.Metrics().AppendEmpty()
	metric4.SetName("response_time")
	metric4.SetUnit("ms")
	metric4.SetEmptyGauge()
	dp4 := metric4.Gauge().DataPoints().AppendEmpty()
	dp4.SetDoubleValue(180.0)
	dp4.SetTimestamp(pcommon.NewTimestampFromTime(testTime))

	return md
}

func countMetrics(md pmetric.Metrics) int {
	count := 0
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			count += sm.Metrics().Len()
		}
	}
	return count
}

var testTime = time.Now()

func TestResourceAttributeGrouping(t *testing.T) {
	// Create test configuration
	cfg := &Config{
		GroupByLabels: []string{"cluster", "service"},
		OutputResourceAttributes: map[string]string{
			"aggregation.level": "cluster",
		},
		AggregationRules: []AggregationRule{
			{
				MetricPattern:    "test_metric",
				MatchType:        "strict",
				OutputMetricName: "aggregated_test_metric",
				AggregationType:  "sum",
				OutputMetricType: "sum",
			},
		},
	}

	processor := newMetricsAggregatorProcessor(cfg, zap.NewNop())

	// Create test metrics with resource-level attributes
	md := pmetric.NewMetrics()

	// Resource 1: cluster=prod, service in datapoint
	rm1 := md.ResourceMetrics().AppendEmpty()
	rm1.Resource().Attributes().PutStr("cluster", "prod")
	rm1.Resource().Attributes().PutStr("region", "us-east") // Additional resource attr not in grouping
	sm1 := rm1.ScopeMetrics().AppendEmpty()
	metric1 := sm1.Metrics().AppendEmpty()
	metric1.SetName("test_metric")
	metric1.SetEmptySum()
	dp1 := metric1.Sum().DataPoints().AppendEmpty()
	dp1.SetDoubleValue(100)
	dp1.Attributes().PutStr("service", "web")
	dp1.SetTimestamp(pcommon.Timestamp(1000000))

	// Resource 2: cluster=prod, service in datapoint
	rm2 := md.ResourceMetrics().AppendEmpty()
	rm2.Resource().Attributes().PutStr("cluster", "prod")
	rm2.Resource().Attributes().PutStr("region", "us-west") // Different region
	sm2 := rm2.ScopeMetrics().AppendEmpty()
	metric2 := sm2.Metrics().AppendEmpty()
	metric2.SetName("test_metric")
	metric2.SetEmptySum()
	dp2 := metric2.Sum().DataPoints().AppendEmpty()
	dp2.SetDoubleValue(150)
	dp2.Attributes().PutStr("service", "web")
	dp2.SetTimestamp(pcommon.Timestamp(2000000))

	// Resource 3: cluster=staging, service in datapoint
	rm3 := md.ResourceMetrics().AppendEmpty()
	rm3.Resource().Attributes().PutStr("cluster", "staging")
	rm3.Resource().Attributes().PutStr("region", "us-east")
	sm3 := rm3.ScopeMetrics().AppendEmpty()
	metric3 := sm3.Metrics().AppendEmpty()
	metric3.SetName("test_metric")
	metric3.SetEmptySum()
	dp3 := metric3.Sum().DataPoints().AppendEmpty()
	dp3.SetDoubleValue(80)
	dp3.Attributes().PutStr("service", "web")
	dp3.SetTimestamp(pcommon.Timestamp(3000000))

	// Process the metrics
	result, err := processor.processMetrics(context.Background(), md)
	require.NoError(t, err)

	// Find all aggregated resources (resources that have metrics)
	var aggregatedResources []pmetric.ResourceMetrics
	for i := 0; i < result.ResourceMetrics().Len(); i++ {
		rm := result.ResourceMetrics().At(i)
		hasMetrics := false
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			if rm.ScopeMetrics().At(j).Metrics().Len() > 0 {
				hasMetrics = true
				break
			}
		}
		if hasMetrics {
			aggregatedResources = append(aggregatedResources, rm)
		}
	}

	// Verify results - should have 2 aggregated resources (one for each cluster)
	assert.Equal(t, 2, len(aggregatedResources))

	// Track which resource contexts we've found
	foundProdResource := false
	foundStagingResource := false

	// Check each aggregated resource
	for _, aggregatedRM := range aggregatedResources {
		assert.Equal(t, 1, aggregatedRM.ScopeMetrics().Len())
		aggregatedSM := aggregatedRM.ScopeMetrics().At(0)
		assert.Equal(t, "metricsaggregator", aggregatedSM.Scope().Name())
		assert.Equal(t, 1, aggregatedSM.Metrics().Len())

		// Check the aggregated metric
		aggregatedMetric := aggregatedSM.Metrics().At(0)
		assert.Equal(t, "aggregated_test_metric", aggregatedMetric.Name())
		assert.Equal(t, pmetric.MetricTypeSum, aggregatedMetric.Type())

		// Each resource should have exactly 1 data point
		dataPoints := aggregatedMetric.Sum().DataPoints()
		assert.Equal(t, 1, dataPoints.Len())

		dp := dataPoints.At(0)

		// Check resource-level attributes (cluster should be at resource level)
		resourceCluster, resourceClusterExists := aggregatedRM.Resource().Attributes().Get("cluster")
		assert.True(t, resourceClusterExists, "Cluster should be set as resource attribute")

		// Check datapoint-level attributes (service should be at datapoint level)
		service, serviceExists := dp.Attributes().Get("service")
		assert.True(t, serviceExists, "Service should be set as datapoint attribute")
		if serviceExists {
			assert.Equal(t, "web", service.AsString())
		}

		// Check values based on cluster (from resource attributes)
		if resourceClusterExists {
			clusterValue := resourceCluster.AsString()
			if clusterValue == "prod" {
				assert.Equal(t, 250.0, dp.DoubleValue()) // 100 + 150
				foundProdResource = true
			} else if clusterValue == "staging" {
				assert.Equal(t, 80.0, dp.DoubleValue())
				foundStagingResource = true
			} else {
				t.Errorf("Unexpected cluster value: %s", clusterValue)
			}
		}
	}

	assert.True(t, foundProdResource, "Should find aggregated resource for prod cluster")
	assert.True(t, foundStagingResource, "Should find aggregated resource for staging cluster")
}

func TestInvalidRegexPattern(t *testing.T) {
	// Test invalid regex pattern handling
	cfg := &Config{
		GroupByLabels: []string{},
		OutputResourceAttributes: map[string]string{
			"aggregation.level": "cluster",
		},
		AggregationRules: []AggregationRule{
			{
				MetricPattern:    "[invalid regex pattern",
				MatchType:        "regex",
				OutputMetricName: "aggregated_metric",
				AggregationType:  "sum",
			},
		},
	}

	processor := newMetricsAggregatorProcessor(cfg, zap.NewNop())

	// Create test metrics
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()

	metric := sm.Metrics().AppendEmpty()
	metric.SetName("test_metric")
	gauge := metric.SetEmptyGauge()
	dp := gauge.DataPoints().AppendEmpty()
	dp.SetDoubleValue(100.0)

	// Process metrics - should not crash and should not match anything
	result, err := processor.processMetrics(context.Background(), md)
	require.NoError(t, err)

	// Should have same number of metrics (no aggregation due to invalid regex)
	originalCount := countMetrics(md)
	resultCount := countMetrics(result)
	assert.Equal(t, originalCount, resultCount, "Invalid regex should not match any metrics")

	// Verify no aggregated metric was created
	foundAggregated := false
	for i := 0; i < result.ResourceMetrics().Len(); i++ {
		rm := result.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				metric := sm.Metrics().At(k)
				if metric.Name() == "aggregated_metric" {
					foundAggregated = true
				}
			}
		}
	}
	assert.False(t, foundAggregated, "No aggregated metric should be created with invalid regex")
}

func TestHistogramMetricAggregation(t *testing.T) {
	cfg := &Config{
		GroupByLabels: []string{},
		OutputResourceAttributes: map[string]string{
			"aggregation.level": "cluster",
		},
		AggregationRules: []AggregationRule{
			{
				MetricPattern:    "request_duration",
				MatchType:        "strict",
				OutputMetricName: "aggregated_request_duration",
				AggregationType:  "sum",
				OutputMetricType: "histogram",
			},
		},
	}

	processor := newMetricsAggregatorProcessor(cfg, zap.NewNop())

	// Create test metrics with histogram data
	md := pmetric.NewMetrics()

	// Resource 1
	rm1 := md.ResourceMetrics().AppendEmpty()
	rm1.Resource().Attributes().PutStr("service", "web")
	sm1 := rm1.ScopeMetrics().AppendEmpty()

	metric1 := sm1.Metrics().AppendEmpty()
	metric1.SetName("request_duration")
	histogram1 := metric1.SetEmptyHistogram()
	dp1 := histogram1.DataPoints().AppendEmpty()
	dp1.SetSum(150.0)
	dp1.SetCount(10)
	dp1.SetTimestamp(pcommon.NewTimestampFromTime(testTime))

	// Add bucket counts
	dp1.BucketCounts().FromRaw([]uint64{2, 3, 4, 1})
	dp1.ExplicitBounds().FromRaw([]float64{10, 50, 100})

	// Resource 2
	rm2 := md.ResourceMetrics().AppendEmpty()
	rm2.Resource().Attributes().PutStr("service", "api")
	sm2 := rm2.ScopeMetrics().AppendEmpty()

	metric2 := sm2.Metrics().AppendEmpty()
	metric2.SetName("request_duration")
	histogram2 := metric2.SetEmptyHistogram()
	dp2 := histogram2.DataPoints().AppendEmpty()
	dp2.SetSum(200.0)
	dp2.SetCount(15)
	dp2.SetTimestamp(pcommon.NewTimestampFromTime(testTime))

	// Add bucket counts
	dp2.BucketCounts().FromRaw([]uint64{1, 5, 7, 2})
	dp2.ExplicitBounds().FromRaw([]float64{10, 50, 100})

	// Process metrics
	result, err := processor.processMetrics(context.Background(), md)
	require.NoError(t, err)

	// Find the aggregated histogram metric
	found := false
	for i := 0; i < result.ResourceMetrics().Len(); i++ {
		rm := result.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				metric := sm.Metrics().At(k)
				if metric.Name() == "aggregated_request_duration" {
					found = true
					assert.Equal(t, pmetric.MetricTypeHistogram, metric.Type())

					dataPoints := metric.Histogram().DataPoints()
					assert.Equal(t, 1, dataPoints.Len())

					dp := dataPoints.At(0)
					// Sum should be aggregated: 150 + 200 = 350
					assert.Equal(t, 350.0, dp.Sum())
					// Count should be number of data points aggregated: 2 (one from each resource)
					assert.Equal(t, uint64(2), dp.Count())
				}
			}
		}
	}
	assert.True(t, found, "Aggregated histogram metric should be found")
}

func TestAlternativeAggregationTypes(t *testing.T) {
	tests := []struct {
		name            string
		aggregationType string
		inputValues     []float64
		expectedValue   float64
	}{
		{
			name:            "min aggregation",
			aggregationType: "min",
			inputValues:     []float64{100.0, 50.0, 200.0, 75.0},
			expectedValue:   50.0,
		},
		{
			name:            "max aggregation",
			aggregationType: "max",
			inputValues:     []float64{100.0, 50.0, 200.0, 75.0},
			expectedValue:   200.0,
		},
		{
			name:            "count aggregation",
			aggregationType: "count",
			inputValues:     []float64{100.0, 50.0, 200.0, 75.0},
			expectedValue:   4.0,
		},
		{
			name:            "mean aggregation",
			aggregationType: "mean",
			inputValues:     []float64{100.0, 50.0, 200.0, 75.0},
			expectedValue:   106.25, // (100 + 50 + 200 + 75) / 4 = 425 / 4 = 106.25
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				GroupByLabels: []string{},
				OutputResourceAttributes: map[string]string{
					"aggregation.type": tt.aggregationType,
				},
				AggregationRules: []AggregationRule{
					{
						MetricPattern:    "test_metric",
						MatchType:        "strict",
						OutputMetricName: "aggregated_metric",
						AggregationType:  tt.aggregationType,
					},
				},
			}

			processor := newMetricsAggregatorProcessor(cfg, zap.NewNop())

			// Create test metrics with specified values
			md := pmetric.NewMetrics()

			for i, value := range tt.inputValues {
				rm := md.ResourceMetrics().AppendEmpty()
				rm.Resource().Attributes().PutStr("instance", fmt.Sprintf("instance-%d", i))
				sm := rm.ScopeMetrics().AppendEmpty()

				metric := sm.Metrics().AppendEmpty()
				metric.SetName("test_metric")
				gauge := metric.SetEmptyGauge()
				dp := gauge.DataPoints().AppendEmpty()
				dp.SetDoubleValue(value)
				dp.SetTimestamp(pcommon.NewTimestampFromTime(testTime))
			}

			// Process metrics
			result, err := processor.processMetrics(context.Background(), md)
			require.NoError(t, err)

			// Find and validate the aggregated metric
			found := false
			for i := 0; i < result.ResourceMetrics().Len(); i++ {
				rm := result.ResourceMetrics().At(i)
				for j := 0; j < rm.ScopeMetrics().Len(); j++ {
					sm := rm.ScopeMetrics().At(j)
					for k := 0; k < sm.Metrics().Len(); k++ {
						metric := sm.Metrics().At(k)
						if metric.Name() == "aggregated_metric" {
							found = true
							assert.Equal(t, pmetric.MetricTypeGauge, metric.Type())

							dataPoints := metric.Gauge().DataPoints()
							assert.Equal(t, 1, dataPoints.Len())

							dp := dataPoints.At(0)
							assert.Equal(t, tt.expectedValue, dp.DoubleValue(),
								"Aggregated value should match expected %s result", tt.aggregationType)
						}
					}
				}
			}
			assert.True(t, found, "Aggregated metric should be found for %s aggregation", tt.aggregationType)
		})
	}
}

func TestMixedValueTypes(t *testing.T) {
	cfg := &Config{
		GroupByLabels: []string{},
		OutputResourceAttributes: map[string]string{
			"aggregation.level": "cluster",
		},
		AggregationRules: []AggregationRule{
			{
				MetricPattern:    "mixed_metric",
				MatchType:        "strict",
				OutputMetricName: "aggregated_mixed_metric",
				AggregationType:  "sum",
			},
		},
	}

	processor := newMetricsAggregatorProcessor(cfg, zap.NewNop())

	// Create test metrics with mixed int and double values
	md := pmetric.NewMetrics()

	// Resource 1 - Double value
	rm1 := md.ResourceMetrics().AppendEmpty()
	rm1.Resource().Attributes().PutStr("instance", "instance-1")
	sm1 := rm1.ScopeMetrics().AppendEmpty()

	metric1 := sm1.Metrics().AppendEmpty()
	metric1.SetName("mixed_metric")
	gauge1 := metric1.SetEmptyGauge()
	dp1 := gauge1.DataPoints().AppendEmpty()
	dp1.SetDoubleValue(100.5) // Double value
	dp1.SetTimestamp(pcommon.NewTimestampFromTime(testTime))

	// Resource 2 - Int value
	rm2 := md.ResourceMetrics().AppendEmpty()
	rm2.Resource().Attributes().PutStr("instance", "instance-2")
	sm2 := rm2.ScopeMetrics().AppendEmpty()

	metric2 := sm2.Metrics().AppendEmpty()
	metric2.SetName("mixed_metric")
	gauge2 := metric2.SetEmptyGauge()
	dp2 := gauge2.DataPoints().AppendEmpty()
	dp2.SetIntValue(50) // Int value
	dp2.SetTimestamp(pcommon.NewTimestampFromTime(testTime))

	// Resource 3 - Another double value
	rm3 := md.ResourceMetrics().AppendEmpty()
	rm3.Resource().Attributes().PutStr("instance", "instance-3")
	sm3 := rm3.ScopeMetrics().AppendEmpty()

	metric3 := sm3.Metrics().AppendEmpty()
	metric3.SetName("mixed_metric")
	gauge3 := metric3.SetEmptyGauge()
	dp3 := gauge3.DataPoints().AppendEmpty()
	dp3.SetDoubleValue(25.3) // Double value
	dp3.SetTimestamp(pcommon.NewTimestampFromTime(testTime))

	// Process metrics
	result, err := processor.processMetrics(context.Background(), md)
	require.NoError(t, err)

	// Find and validate the aggregated metric
	found := false
	for i := 0; i < result.ResourceMetrics().Len(); i++ {
		rm := result.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				metric := sm.Metrics().At(k)
				if metric.Name() == "aggregated_mixed_metric" {
					found = true
					assert.Equal(t, pmetric.MetricTypeGauge, metric.Type())

					dataPoints := metric.Gauge().DataPoints()
					assert.Equal(t, 1, dataPoints.Len())

					dp := dataPoints.At(0)
					// Sum should be: 100.5 + 50.0 + 25.3 = 175.8
					assert.Equal(t, 175.8, dp.DoubleValue(), "Mixed int/double values should be summed correctly")
				}
			}
		}
	}
	assert.True(t, found, "Aggregated metric should be found for mixed value types")
}

func TestEmptyValuesArray(t *testing.T) {
	cfg := &Config{
		GroupByLabels: []string{},
		OutputResourceAttributes: map[string]string{
			"aggregation.level": "cluster",
		},
		AggregationRules: []AggregationRule{
			{
				MetricPattern:    "nonexistent_metric",
				MatchType:        "strict",
				OutputMetricName: "aggregated_empty_metric",
				AggregationType:  "sum",
			},
		},
	}

	processor := newMetricsAggregatorProcessor(cfg, zap.NewNop())

	// Create test metrics that won't match the pattern
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()

	metric := sm.Metrics().AppendEmpty()
	metric.SetName("different_metric") // Won't match "nonexistent_metric"
	gauge := metric.SetEmptyGauge()
	dp := gauge.DataPoints().AppendEmpty()
	dp.SetDoubleValue(100.0)

	// Process metrics
	result, err := processor.processMetrics(context.Background(), md)
	require.NoError(t, err)

	// Should have same number of metrics (no aggregation occurred)
	originalCount := countMetrics(md)
	resultCount := countMetrics(result)
	assert.Equal(t, originalCount, resultCount, "No aggregation should occur when no metrics match")

	// Verify no aggregated metric was created
	foundAggregated := false
	for i := 0; i < result.ResourceMetrics().Len(); i++ {
		rm := result.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				metric := sm.Metrics().At(k)
				if metric.Name() == "aggregated_empty_metric" {
					foundAggregated = true
				}
			}
		}
	}
	assert.False(t, foundAggregated, "No aggregated metric should be created when no values to aggregate")
}

func TestUnknownAggregationType(t *testing.T) {
	cfg := &Config{
		GroupByLabels: []string{},
		OutputResourceAttributes: map[string]string{
			"aggregation.level": "cluster",
		},
		AggregationRules: []AggregationRule{
			{
				MetricPattern:    "test_metric",
				MatchType:        "strict",
				OutputMetricName: "aggregated_metric",
				AggregationType:  "unknown_type", // Invalid aggregation type
			},
		},
	}

	processor := newMetricsAggregatorProcessor(cfg, zap.NewNop())

	// Create test metrics
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()

	metric := sm.Metrics().AppendEmpty()
	metric.SetName("test_metric")
	gauge := metric.SetEmptyGauge()
	dp := gauge.DataPoints().AppendEmpty()
	dp.SetDoubleValue(100.0)

	// Process metrics
	result, err := processor.processMetrics(context.Background(), md)
	require.NoError(t, err)

	// Find the aggregated metric
	found := false
	for i := 0; i < result.ResourceMetrics().Len(); i++ {
		rm := result.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				metric := sm.Metrics().At(k)
				if metric.Name() == "aggregated_metric" {
					found = true
					assert.Equal(t, pmetric.MetricTypeGauge, metric.Type())

					dataPoints := metric.Gauge().DataPoints()
					assert.Equal(t, 1, dataPoints.Len())

					dp := dataPoints.At(0)
					// Unknown aggregation type should default to 0
					assert.Equal(t, 0.0, dp.DoubleValue(), "Unknown aggregation type should return 0")
				}
			}
		}
	}
	assert.True(t, found, "Aggregated metric should be created even with unknown aggregation type")
}

// CRITICAL: Test smart label filtering - the core new feature
func TestSmartLabelFiltering(t *testing.T) {
	tests := []struct {
		name             string
		groupByLabels    []string
		resourceAttrs    map[string]string
		datapointAttrs   map[string]string
		expectedGroupKey string
	}{
		{
			name:             "missing labels excluded",
			groupByLabels:    []string{"cluster", "service", "missing_label"},
			resourceAttrs:    map[string]string{"cluster": "prod"},
			datapointAttrs:   map[string]string{"service": "web"},
			expectedGroupKey: "cluster=prod|service=web",
		},
		{
			name:             "empty string labels included",
			groupByLabels:    []string{"cluster", "service", "empty_label"},
			resourceAttrs:    map[string]string{"cluster": "prod", "empty_label": ""},
			datapointAttrs:   map[string]string{"service": "web"},
			expectedGroupKey: "cluster=prod|service=web|empty_label=",
		},
		{
			name:             "datapoint overrides resource",
			groupByLabels:    []string{"environment"},
			resourceAttrs:    map[string]string{"environment": "resource_env"},
			datapointAttrs:   map[string]string{"environment": "datapoint_env"},
			expectedGroupKey: "environment=datapoint_env",
		},
		{
			name:             "no labels present",
			groupByLabels:    []string{"missing1", "missing2"},
			resourceAttrs:    map[string]string{"other": "value"},
			datapointAttrs:   map[string]string{"other2": "value2"},
			expectedGroupKey: "all",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				GroupByLabels: tt.groupByLabels,
				OutputResourceAttributes: map[string]string{
					"aggregation.test": "true",
				},
				AggregationRules: []AggregationRule{
					{
						MetricPattern:    "test_metric",
						MatchType:        "strict",
						OutputMetricName: "aggregated_metric",
						AggregationType:  "sum",
					},
				},
			}

			processor := newMetricsAggregatorProcessor(cfg, zap.NewNop())

			// Test the buildGroupKeyFromBothAttributes function directly
			resourceAttrs := pcommon.NewMap()
			for k, v := range tt.resourceAttrs {
				resourceAttrs.PutStr(k, v)
			}

			datapointAttrs := pcommon.NewMap()
			for k, v := range tt.datapointAttrs {
				datapointAttrs.PutStr(k, v)
			}

			groupKey := processor.buildGroupKeyFromPresentAttributes(resourceAttrs, datapointAttrs, tt.groupByLabels)

			assert.Equal(t, tt.expectedGroupKey, groupKey, "Group key should match expected")
		})
	}
}

// Test PreserveOriginalMetrics behavior
func TestPreserveOriginalMetrics(t *testing.T) {
	tests := []struct {
		name                    string
		preserveOriginalMetrics bool
		expectedOriginalCount   int
		expectedAggregatedCount int
	}{
		{
			name:                    "preserve original metrics",
			preserveOriginalMetrics: true,
			expectedOriginalCount:   2, // Both original metrics should remain
			expectedAggregatedCount: 1, // Plus one aggregated
		},
		{
			name:                    "remove original metrics",
			preserveOriginalMetrics: false,
			expectedOriginalCount:   0, // Original metrics should be removed
			expectedAggregatedCount: 1, // Only aggregated remains
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				GroupByLabels: []string{},
				OutputResourceAttributes: map[string]string{
					"aggregated": "true",
				},
				AggregationRules: []AggregationRule{
					{
						MetricPattern:           "test_metric",
						MatchType:               "strict",
						OutputMetricName:        "aggregated_metric",
						AggregationType:         "sum",
						PreserveOriginalMetrics: tt.preserveOriginalMetrics,
					},
				},
			}

			processor := newMetricsAggregatorProcessor(cfg, zap.NewNop())

			// Create test metrics
			md := pmetric.NewMetrics()

			// Resource 1
			rm1 := md.ResourceMetrics().AppendEmpty()
			rm1.Resource().Attributes().PutStr("instance", "instance-1")
			sm1 := rm1.ScopeMetrics().AppendEmpty()
			metric1 := sm1.Metrics().AppendEmpty()
			metric1.SetName("test_metric")
			gauge1 := metric1.SetEmptyGauge()
			dp1 := gauge1.DataPoints().AppendEmpty()
			dp1.SetDoubleValue(100.0)

			// Resource 2
			rm2 := md.ResourceMetrics().AppendEmpty()
			rm2.Resource().Attributes().PutStr("instance", "instance-2")
			sm2 := rm2.ScopeMetrics().AppendEmpty()
			metric2 := sm2.Metrics().AppendEmpty()
			metric2.SetName("test_metric")
			gauge2 := metric2.SetEmptyGauge()
			dp2 := gauge2.DataPoints().AppendEmpty()
			dp2.SetDoubleValue(200.0)

			// Process metrics
			result, err := processor.processMetrics(context.Background(), md)
			require.NoError(t, err)

			// Count original and aggregated metrics
			originalCount := 0
			aggregatedCount := 0

			for i := 0; i < result.ResourceMetrics().Len(); i++ {
				rm := result.ResourceMetrics().At(i)

				// Check if this is an aggregated resource
				isAggregated := false
				if val, exists := rm.Resource().Attributes().Get("aggregated"); exists && val.AsString() == "true" {
					isAggregated = true
				}

				for j := 0; j < rm.ScopeMetrics().Len(); j++ {
					sm := rm.ScopeMetrics().At(j)
					for k := 0; k < sm.Metrics().Len(); k++ {
						if isAggregated {
							aggregatedCount++
						} else {
							originalCount++
						}
					}
				}
			}

			assert.Equal(t, tt.expectedOriginalCount, originalCount, "Original metric count should match expected")
			assert.Equal(t, tt.expectedAggregatedCount, aggregatedCount, "Aggregated metric count should match expected")
		})
	}
}

// Test edge cases with zero and negative values
func TestZeroAndNegativeValues(t *testing.T) {
	cfg := &Config{
		GroupByLabels: []string{},
		OutputResourceAttributes: map[string]string{
			"aggregation.test": "true",
		},
		AggregationRules: []AggregationRule{
			{
				MetricPattern:    "test_metric",
				MatchType:        "strict",
				OutputMetricName: "aggregated_metric",
				AggregationType:  "sum",
			},
		},
	}

	processor := newMetricsAggregatorProcessor(cfg, zap.NewNop())

	// Create test metrics with zero and negative values
	md := pmetric.NewMetrics()

	values := []float64{0.0, -100.0, 150.0, -50.0}
	for i, value := range values {
		rm := md.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().PutStr("instance", fmt.Sprintf("instance-%d", i))
		sm := rm.ScopeMetrics().AppendEmpty()
		metric := sm.Metrics().AppendEmpty()
		metric.SetName("test_metric")
		gauge := metric.SetEmptyGauge()
		dp := gauge.DataPoints().AppendEmpty()
		dp.SetDoubleValue(value)
	}

	// Process metrics
	result, err := processor.processMetrics(context.Background(), md)
	require.NoError(t, err)

	// Find and validate the aggregated metric
	found := false
	for i := 0; i < result.ResourceMetrics().Len(); i++ {
		rm := result.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				metric := sm.Metrics().At(k)
				if metric.Name() == "aggregated_metric" {
					found = true
					dataPoints := metric.Gauge().DataPoints()
					assert.Equal(t, 1, dataPoints.Len())

					dp := dataPoints.At(0)
					expectedSum := 0.0 + (-100.0) + 150.0 + (-50.0) // = 0.0
					assert.Equal(t, expectedSum, dp.DoubleValue(), "Should handle zero and negative values correctly")
				}
			}
		}
	}
	assert.True(t, found, "Aggregated metric should be found")
}

// Test multiple datapoints per metric
func TestMultipleDataPointsPerMetric(t *testing.T) {
	cfg := &Config{
		GroupByLabels: []string{"service"},
		OutputResourceAttributes: map[string]string{
			"aggregation.test": "true",
		},
		AggregationRules: []AggregationRule{
			{
				MetricPattern:    "test_metric",
				MatchType:        "strict",
				OutputMetricName: "aggregated_metric",
				AggregationType:  "sum",
			},
		},
	}

	processor := newMetricsAggregatorProcessor(cfg, zap.NewNop())

	// Create test metrics with multiple datapoints
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("cluster", "prod")
	sm := rm.ScopeMetrics().AppendEmpty()

	metric := sm.Metrics().AppendEmpty()
	metric.SetName("test_metric")
	gauge := metric.SetEmptyGauge()

	// Add multiple datapoints with different service labels
	dp1 := gauge.DataPoints().AppendEmpty()
	dp1.SetDoubleValue(100.0)
	dp1.Attributes().PutStr("service", "web")

	dp2 := gauge.DataPoints().AppendEmpty()
	dp2.SetDoubleValue(150.0)
	dp2.Attributes().PutStr("service", "web")

	dp3 := gauge.DataPoints().AppendEmpty()
	dp3.SetDoubleValue(200.0)
	dp3.Attributes().PutStr("service", "api")

	// Process metrics
	result, err := processor.processMetrics(context.Background(), md)
	require.NoError(t, err)

	// Should create 2 groups: web and api, but values need to be checked from actual output
	webFound := false
	apiFound := false

	for i := 0; i < result.ResourceMetrics().Len(); i++ {
		rm := result.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				metric := sm.Metrics().At(k)
				if metric.Name() == "aggregated_metric" {
					dataPoints := metric.Gauge().DataPoints()
					for l := 0; l < dataPoints.Len(); l++ {
						dp := dataPoints.At(l)
						if service, exists := dp.Attributes().Get("service"); exists {
							switch service.AsString() {
							case "web":
								// Web service found (value depends on processor implementation)
								webFound = true
							case "api":
								// API service found (value depends on processor implementation)
								apiFound = true
							}
						}
					}
				}
			}
		}
	}

	assert.True(t, webFound, "Should find aggregated web service metric")
	assert.True(t, apiFound, "Should find aggregated api service metric")
}

// Test to verify correct aggregation with multiple datapoints per metric
func TestMultipleDatapointsPerMetricCorrectAggregation(t *testing.T) {
	cfg := &Config{
		GroupByLabels: []string{"service"},
		OutputResourceAttributes: map[string]string{
			"aggregation.test": "true",
		},
		AggregationRules: []AggregationRule{
			{
				MetricPattern:    "test_metric",
				MatchType:        "strict",
				OutputMetricName: "aggregated_metric",
				AggregationType:  "sum",
			},
		},
	}

	processor := newMetricsAggregatorProcessor(cfg, zap.NewNop())

	// Create a single metric with multiple datapoints having different service labels
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("cluster", "prod")
	sm := rm.ScopeMetrics().AppendEmpty()

	metric := sm.Metrics().AppendEmpty()
	metric.SetName("test_metric")
	gauge := metric.SetEmptyGauge()

	// Add datapoints with different service labels
	dp1 := gauge.DataPoints().AppendEmpty()
	dp1.SetDoubleValue(100.0)
	dp1.Attributes().PutStr("service", "web")

	dp2 := gauge.DataPoints().AppendEmpty()
	dp2.SetDoubleValue(200.0)
	dp2.Attributes().PutStr("service", "api")

	dp3 := gauge.DataPoints().AppendEmpty()
	dp3.SetDoubleValue(300.0)
	dp3.Attributes().PutStr("service", "web")

	// Process metrics
	result, err := processor.processMetrics(context.Background(), md)
	require.NoError(t, err)

	// Count total aggregated values
	totalAggregatedValue := 0.0
	aggregatedMetricCount := 0

	for i := 0; i < result.ResourceMetrics().Len(); i++ {
		rm := result.ResourceMetrics().At(i)
		// Check if this is an aggregated resource
		if val, exists := rm.Resource().Attributes().Get("aggregation.test"); exists && val.AsString() == "true" {
			for j := 0; j < rm.ScopeMetrics().Len(); j++ {
				sm := rm.ScopeMetrics().At(j)
				for k := 0; k < sm.Metrics().Len(); k++ {
					metric := sm.Metrics().At(k)
					if metric.Name() == "aggregated_metric" {
						dataPoints := metric.Gauge().DataPoints()
						for l := 0; l < dataPoints.Len(); l++ {
							dp := dataPoints.At(l)
							totalAggregatedValue += dp.DoubleValue()
							aggregatedMetricCount++

							// Print for debugging
							service := "unknown"
							if serviceAttr, exists := dp.Attributes().Get("service"); exists {
								service = serviceAttr.AsString()
							}
							t.Logf("Aggregated datapoint: service=%s, value=%f", service, dp.DoubleValue())
						}
					}
				}
			}
		}
	}

	// Expected: 2 groups (web=400, api=200), total should be 600
	// Actual: May be much higher due to value duplication
	expectedTotal := 600.0 // (100 + 300) + 200

	t.Logf("Total aggregated value: %f, Expected: %f", totalAggregatedValue, expectedTotal)
	t.Logf("Aggregated metric count: %d, Expected: 2", aggregatedMetricCount)

	// Verify correct aggregation behavior
	assert.Equal(t, expectedTotal, totalAggregatedValue, "Aggregated values should be correct")
	assert.Equal(t, 2, aggregatedMetricCount, "Should have 2 aggregated groups (web and api)")
}

// Test metric name sanitization
func TestMetricNameSanitization(t *testing.T) {
	tests := []struct {
		name                  string
		inputMetricName       string
		outputMetricName      string
		expectedSanitizedName string
	}{
		{
			name:                  "invalid characters",
			inputMetricName:       "test-metric.with@invalid#chars",
			outputMetricName:      "sanitized-metric.name@test#",
			expectedSanitizedName: "sanitized_metric_name_test_",
		},
		{
			name:                  "starts with number",
			inputMetricName:       "test_metric",
			outputMetricName:      "123_invalid_start",
			expectedSanitizedName: "_123_invalid_start",
		},
		{
			name:                  "valid name unchanged",
			inputMetricName:       "test_metric",
			outputMetricName:      "valid_metric_name",
			expectedSanitizedName: "valid_metric_name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				GroupByLabels: []string{},
				OutputResourceAttributes: map[string]string{
					"test": "sanitization",
				},
				AggregationRules: []AggregationRule{
					{
						MetricPattern:    tt.inputMetricName,
						MatchType:        "strict",
						OutputMetricName: tt.outputMetricName,
						AggregationType:  "sum",
					},
				},
			}

			processor := newMetricsAggregatorProcessor(cfg, zap.NewNop())

			// Create test metric
			md := pmetric.NewMetrics()
			rm := md.ResourceMetrics().AppendEmpty()
			sm := rm.ScopeMetrics().AppendEmpty()
			metric := sm.Metrics().AppendEmpty()
			metric.SetName(tt.inputMetricName)
			gauge := metric.SetEmptyGauge()
			dp := gauge.DataPoints().AppendEmpty()
			dp.SetDoubleValue(100.0)

			// Process metrics
			result, err := processor.processMetrics(context.Background(), md)
			require.NoError(t, err)

			// Verify sanitized metric name
			found := false
			for i := 0; i < result.ResourceMetrics().Len(); i++ {
				rm := result.ResourceMetrics().At(i)
				if val, exists := rm.Resource().Attributes().Get("test"); exists && val.AsString() == "sanitization" {
					for j := 0; j < rm.ScopeMetrics().Len(); j++ {
						sm := rm.ScopeMetrics().At(j)
						for k := 0; k < sm.Metrics().Len(); k++ {
							metric := sm.Metrics().At(k)
							if metric.Name() == tt.expectedSanitizedName {
								found = true
							}
						}
					}
				}
			}
			assert.True(t, found, "Should find metric with sanitized name: %s", tt.expectedSanitizedName)
		})
	}
}

// Test timestamp handling
func TestTimestampHandling(t *testing.T) {
	cfg := &Config{
		GroupByLabels: []string{},
		OutputResourceAttributes: map[string]string{
			"test": "timestamps",
		},
		AggregationRules: []AggregationRule{
			{
				MetricPattern:    "timestamp_test",
				MatchType:        "strict",
				OutputMetricName: "aggregated_timestamp_test",
				AggregationType:  "sum",
				OutputMetricType: "sum",
			},
		},
	}

	processor := newMetricsAggregatorProcessor(cfg, zap.NewNop())

	now := time.Now()
	earliest := now.Add(-5 * time.Minute)
	latest := now

	// Create test metrics with different timestamps
	md := pmetric.NewMetrics()

	// Metric 1 - earliest timestamp
	rm1 := md.ResourceMetrics().AppendEmpty()
	sm1 := rm1.ScopeMetrics().AppendEmpty()
	metric1 := sm1.Metrics().AppendEmpty()
	metric1.SetName("timestamp_test")
	sum1 := metric1.SetEmptySum()
	sum1.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	dp1 := sum1.DataPoints().AppendEmpty()
	dp1.SetDoubleValue(100.0)
	dp1.SetTimestamp(pcommon.NewTimestampFromTime(earliest))
	dp1.SetStartTimestamp(pcommon.NewTimestampFromTime(earliest.Add(-time.Minute)))

	// Metric 2 - latest timestamp
	rm2 := md.ResourceMetrics().AppendEmpty()
	sm2 := rm2.ScopeMetrics().AppendEmpty()
	metric2 := sm2.Metrics().AppendEmpty()
	metric2.SetName("timestamp_test")
	sum2 := metric2.SetEmptySum()
	sum2.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	dp2 := sum2.DataPoints().AppendEmpty()
	dp2.SetDoubleValue(200.0)
	dp2.SetTimestamp(pcommon.NewTimestampFromTime(latest))
	dp2.SetStartTimestamp(pcommon.NewTimestampFromTime(latest.Add(-time.Minute)))

	// Process metrics
	result, err := processor.processMetrics(context.Background(), md)
	require.NoError(t, err)

	// Verify timestamp handling
	found := false
	for i := 0; i < result.ResourceMetrics().Len(); i++ {
		rm := result.ResourceMetrics().At(i)
		if val, exists := rm.Resource().Attributes().Get("test"); exists && val.AsString() == "timestamps" {
			for j := 0; j < rm.ScopeMetrics().Len(); j++ {
				sm := rm.ScopeMetrics().At(j)
				for k := 0; k < sm.Metrics().Len(); k++ {
					metric := sm.Metrics().At(k)
					if metric.Name() == "aggregated_timestamp_test" {
						found = true
						assert.Equal(t, pmetric.MetricTypeSum, metric.Type())

						dataPoints := metric.Sum().DataPoints()
						assert.Equal(t, 1, dataPoints.Len())

						dp := dataPoints.At(0)
						assert.Equal(t, 300.0, dp.DoubleValue(), "Should sum values correctly")

						// Should use latest timestamp
						actualTimestamp := dp.Timestamp()
						expectedTimestamp := pcommon.NewTimestampFromTime(latest)
						assert.Equal(t, expectedTimestamp, actualTimestamp, "Should use latest timestamp")

						// Should use earliest start timestamp
						actualStartTimestamp := dp.StartTimestamp()
						expectedStartTimestamp := pcommon.NewTimestampFromTime(earliest.Add(-time.Minute))
						assert.Equal(t, expectedStartTimestamp, actualStartTimestamp, "Should use earliest start timestamp")
					}
				}
			}
		}
	}
	assert.True(t, found, "Should find aggregated metric with correct timestamps")
}

// Test invalid match type
func TestInvalidMatchType(t *testing.T) {
	cfg := &Config{
		GroupByLabels: []string{},
		OutputResourceAttributes: map[string]string{
			"test": "invalid_match",
		},
		AggregationRules: []AggregationRule{
			{
				MetricPattern:    "test_metric",
				MatchType:        "invalid_match_type",
				OutputMetricName: "should_not_match",
				AggregationType:  "sum",
			},
		},
	}

	processor := newMetricsAggregatorProcessor(cfg, zap.NewNop())

	// Create test metric
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("test_metric")
	gauge := metric.SetEmptyGauge()
	dp := gauge.DataPoints().AppendEmpty()
	dp.SetDoubleValue(100.0)

	originalCount := countMetrics(md)

	// Process metrics
	result, err := processor.processMetrics(context.Background(), md)
	require.NoError(t, err)

	// Should not create any aggregated metrics due to invalid match type
	resultCount := countMetrics(result)
	assert.Equal(t, originalCount, resultCount, "Should not aggregate with invalid match type")

	// Verify no aggregated resource was created
	foundAggregated := false
	for i := 0; i < result.ResourceMetrics().Len(); i++ {
		rm := result.ResourceMetrics().At(i)
		if val, exists := rm.Resource().Attributes().Get("test"); exists && val.AsString() == "invalid_match" {
			foundAggregated = true
		}
	}
	assert.False(t, foundAggregated, "Should not create aggregated resource with invalid match type")
}

// Test empty group by labels explicitly
func TestEmptyGroupByLabels(t *testing.T) {
	cfg := &Config{
		GroupByLabels: []string{}, // Explicitly empty
		OutputResourceAttributes: map[string]string{
			"grouping": "none",
		},
		AggregationRules: []AggregationRule{
			{
				MetricPattern:    "test_metric",
				MatchType:        "strict",
				OutputMetricName: "aggregated_no_grouping",
				AggregationType:  "sum",
			},
		},
	}

	processor := newMetricsAggregatorProcessor(cfg, zap.NewNop())

	// Create multiple metrics with different labels
	md := pmetric.NewMetrics()

	values := []float64{100.0, 200.0, 300.0}
	services := []string{"web", "api", "db"}

	for i, value := range values {
		rm := md.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().PutStr("service", services[i])
		sm := rm.ScopeMetrics().AppendEmpty()
		metric := sm.Metrics().AppendEmpty()
		metric.SetName("test_metric")
		gauge := metric.SetEmptyGauge()
		dp := gauge.DataPoints().AppendEmpty()
		dp.SetDoubleValue(value)
		dp.Attributes().PutStr("service", services[i])
	}

	// Process metrics
	result, err := processor.processMetrics(context.Background(), md)
	require.NoError(t, err)

	// Should create single aggregated metric (no grouping)
	found := false
	for i := 0; i < result.ResourceMetrics().Len(); i++ {
		rm := result.ResourceMetrics().At(i)
		if val, exists := rm.Resource().Attributes().Get("grouping"); exists && val.AsString() == "none" {
			for j := 0; j < rm.ScopeMetrics().Len(); j++ {
				sm := rm.ScopeMetrics().At(j)
				for k := 0; k < sm.Metrics().Len(); k++ {
					metric := sm.Metrics().At(k)
					if metric.Name() == "aggregated_no_grouping" {
						found = true

						dataPoints := metric.Gauge().DataPoints()
						assert.Equal(t, 1, dataPoints.Len(), "Should have single aggregated datapoint")

						dp := dataPoints.At(0)
						expectedSum := 600.0 // 100 + 200 + 300
						assert.Equal(t, expectedSum, dp.DoubleValue(), "Should aggregate all values into single group")

						// Should not have any grouping labels
						assert.Equal(t, 0, dp.Attributes().Len(), "Should not have any labels with empty grouping")
					}
				}
			}
		}
	}
	assert.True(t, found, "Should find single aggregated metric with no grouping")
}
