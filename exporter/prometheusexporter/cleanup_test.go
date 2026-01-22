// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusexporter

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/otel/semconv/v1.25.0"
	"go.uber.org/zap"
)

func TestAccumulatorCleanupMethods(t *testing.T) {
	logger := zap.NewNop()
	acc := newAccumulator(logger, time.Minute*5).(*lastValueAccumulator)

	// Create test metrics with different labels
	rm1 := createTestResourceMetrics("test_metric_1", "test-job", "test-instance-1", map[string]interface{}{
		"service":     "web",
		"environment": "production",
	})
	rm2 := createTestResourceMetrics("test_metric_2", "test-job", "test-instance-2", map[string]interface{}{
		"service":     "db",
		"environment": "production",
	})
	rm3 := createTestResourceMetrics("another_metric", "other-job", "test-instance-1", map[string]interface{}{
		"service":     "web",
		"environment": "staging",
	})

	// Accumulate metrics
	acc.Accumulate(rm1)
	acc.Accumulate(rm2)
	acc.Accumulate(rm3)

	// Verify initial count
	metrics, _, _, _, _, _ := acc.Collect()
	assert.Equal(t, 3, len(metrics))

	t.Run("CleanByLabels", func(t *testing.T) {
		// Clean metrics with service.name="test-job"
		deleted := acc.CleanByLabels(map[string]string{
			string(conventions.ServiceNameKey): "test-job",
		})
		assert.Equal(t, 2, deleted)

		// Verify remaining metrics
		metrics, _, _, _, _, _ := acc.Collect()
		assert.Equal(t, 1, len(metrics))
		assert.Equal(t, "another_metric", metrics[0].Name())
	})

	// Reset for next test
	acc = newAccumulator(logger, time.Minute*5).(*lastValueAccumulator)
	acc.Accumulate(rm1)
	acc.Accumulate(rm2)
	acc.Accumulate(rm3)

	t.Run("CleanByMetricName", func(t *testing.T) {
		// Clean metrics matching pattern "test_metric_*"
		deleted := acc.CleanByMetricName("test_metric_")
		assert.Equal(t, 2, deleted)

		// Verify remaining metrics
		metrics, _, _, _, _, _ := acc.Collect()
		assert.Equal(t, 1, len(metrics))
		assert.Equal(t, "another_metric", metrics[0].Name())
	})

	t.Run("CleanByMetricNameRegex", func(t *testing.T) {
		// Reset
		acc = newAccumulator(logger, time.Minute*5).(*lastValueAccumulator)
		acc.Accumulate(rm1)
		acc.Accumulate(rm2)
		acc.Accumulate(rm3)

		// Clean metrics using regex pattern
		deleted := acc.CleanByMetricName("test_metric_.*")
		assert.Equal(t, 2, deleted)

		// Verify remaining metrics
		metrics, _, _, _, _, _ := acc.Collect()
		assert.Equal(t, 1, len(metrics))
		assert.Equal(t, "another_metric", metrics[0].Name())
	})

	t.Run("CleanExpired", func(t *testing.T) {
		// Create accumulator with very short expiration
		shortExpiration := newAccumulator(logger, time.Millisecond*10).(*lastValueAccumulator)
		shortExpiration.Accumulate(rm1)

		// Wait for expiration
		time.Sleep(time.Millisecond * 20)

		deleted := shortExpiration.CleanExpired()
		assert.Equal(t, 1, deleted)

		// Verify no metrics remain
		metrics, _, _, _, _, _ := shortExpiration.Collect()
		assert.Equal(t, 0, len(metrics))
	})
}

func TestCleanupAPI(t *testing.T) {
	config := createDefaultConfig().(*Config)
	config.ServerConfig.Endpoint = "localhost:0"
	config.EnableCleanupAPI = true

	exporter, err := newPrometheusExporter(config, exportertest.NewNopSettings(component.MustNewType("prometheus")))
	require.NoError(t, err)

	cleanupAPI := NewCleanupAPI(exporter, zap.NewNop())

	// Consume empty metrics to initialize
	err = exporter.ConsumeMetrics(nil, pmetric.NewMetrics())
	require.NoError(t, err)

	t.Run("StatusHandler", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/cleanup/status", nil)
		w := httptest.NewRecorder()

		cleanupAPI.StatusHandler(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "1.0", response["cleanup_api_version"])
		assert.Contains(t, response, "supported_operations")
	})

	t.Run("CleanupHandler_Labels", func(t *testing.T) {
		reqBody := CleanupRequest{
			Type: "labels",
			Filters: map[string]string{
				"job": "test-job",
			},
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/cleanup", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		cleanupAPI.CleanupHandler(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response CleanupResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response.Success)
		assert.Contains(t, response.Message, "Successfully deleted")
	})

	t.Run("CleanupHandler_InvalidMethod", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/cleanup", nil)
		w := httptest.NewRecorder()

		cleanupAPI.CleanupHandler(w, req)

		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)

		var response CleanupResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.False(t, response.Success)
	})

	t.Run("MetricsHandler", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/cleanup/metrics", nil)
		w := httptest.NewRecorder()

		cleanupAPI.MetricsHandler(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Contains(t, response, "current_metric_count")
		assert.Contains(t, response, "timestamp")
	})
}

func TestLabelExtraction(t *testing.T) {
	logger := zap.NewNop()
	acc := newAccumulator(logger, time.Minute*5).(*lastValueAccumulator)

	// Create test metric with complex labels
	rm := createTestResourceMetrics("test_metric", "test-job", "test-instance", map[string]interface{}{
		"service":     "web",
		"environment": "production",
		"version":     "1.0.0",
	})

	acc.Accumulate(rm)

	// Get accumulated value
	var testAccValue *accumulatedValue
	acc.registeredMetrics.Range(func(key, value any) bool {
		testAccValue = value.(*accumulatedValue)
		return false // stop after first item
	})

	require.NotNil(t, testAccValue)

	labels := acc.extractLabelsFromMetric("", testAccValue)

	// Verify extracted labels
	assert.Equal(t, "test-job", labels[string(conventions.ServiceNameKey)])
	assert.Equal(t, "test-instance", labels[string(conventions.ServiceInstanceIDKey)])
	assert.Equal(t, "web", labels["service"])
	assert.Equal(t, "production", labels["environment"])
	assert.Equal(t, "1.0.0", labels["version"])
}

func TestResourceAttributeExtraction(t *testing.T) {
	logger := zap.NewNop()
	acc := newAccumulator(logger, time.Minute*5).(*lastValueAccumulator)

	// Create test metric with comprehensive resource attributes
	rm := createTestResourceMetricsWithResourceAttrs("test_metric", map[string]interface{}{
		// Standard service attributes
		string(conventions.ServiceNameKey):       "payment-service",
		string(conventions.ServiceInstanceIDKey): "instance-123",
		string(conventions.ServiceVersionKey):    "v1.2.3",
		string(conventions.ServiceNamespaceKey):  "production",
		// Custom resource attributes
		"deployment.environment": "staging",
		"k8s.cluster.name":       "prod-cluster",
		"k8s.namespace.name":     "payments",
		"k8s.pod.name":           "payment-pod-abc",
		"cloud.provider":         "aws",
		"cloud.region":           "us-west-2",
	}, map[string]interface{}{
		"method": "POST",
		"status": "200",
	})

	acc.Accumulate(rm)

	// Get accumulated value
	var testAccValue *accumulatedValue
	acc.registeredMetrics.Range(func(key, value any) bool {
		testAccValue = value.(*accumulatedValue)
		return false // stop after first item
	})

	require.NotNil(t, testAccValue)

	labels := acc.extractLabelsFromMetric("", testAccValue)

	// Verify ALL resource attributes are extracted
	assert.Equal(t, "payment-service", labels[string(conventions.ServiceNameKey)])
	assert.Equal(t, "instance-123", labels[string(conventions.ServiceInstanceIDKey)])
	assert.Equal(t, "v1.2.3", labels[string(conventions.ServiceVersionKey)])
	assert.Equal(t, "production", labels[string(conventions.ServiceNamespaceKey)])
	assert.Equal(t, "staging", labels["deployment.environment"])
	assert.Equal(t, "prod-cluster", labels["k8s.cluster.name"])
	assert.Equal(t, "payments", labels["k8s.namespace.name"])
	assert.Equal(t, "payment-pod-abc", labels["k8s.pod.name"])
	assert.Equal(t, "aws", labels["cloud.provider"])
	assert.Equal(t, "us-west-2", labels["cloud.region"])

	// Verify datapoint attributes are also extracted
	assert.Equal(t, "POST", labels["method"])
	assert.Equal(t, "200", labels["status"])
}

func TestCleanupByResourceAttributes(t *testing.T) {
	logger := zap.NewNop()
	acc := newAccumulator(logger, time.Minute*5).(*lastValueAccumulator)

	// Create metrics with different resource attributes
	rm1 := createTestResourceMetricsWithResourceAttrs("metric1", map[string]interface{}{
		string(conventions.ServiceNameKey):       "service-a",
		string(conventions.ServiceInstanceIDKey): "instance-1",
		"deployment.environment":                 "staging",
		"k8s.cluster.name":                       "test-cluster",
	}, map[string]interface{}{"method": "GET"})

	rm2 := createTestResourceMetricsWithResourceAttrs("metric2", map[string]interface{}{
		string(conventions.ServiceNameKey):       "service-b",
		string(conventions.ServiceInstanceIDKey): "instance-2",
		"deployment.environment":                 "production",
		"k8s.cluster.name":                       "prod-cluster",
	}, map[string]interface{}{"method": "POST"})

	rm3 := createTestResourceMetricsWithResourceAttrs("metric3", map[string]interface{}{
		string(conventions.ServiceNameKey):       "service-c",
		string(conventions.ServiceInstanceIDKey): "instance-3",
		"deployment.environment":                 "staging",
		"k8s.cluster.name":                       "test-cluster",
	}, map[string]interface{}{"method": "PUT"})

	// Accumulate metrics
	acc.Accumulate(rm1)
	acc.Accumulate(rm2)
	acc.Accumulate(rm3)

	// Verify initial count
	metrics, _, _, _, _, _ := acc.Collect()
	assert.Equal(t, 3, len(metrics))

	t.Run("CleanByResourceAttribute_Environment", func(t *testing.T) {
		// Clean metrics with deployment.environment="staging"
		deleted := acc.CleanByLabels(map[string]string{
			"deployment.environment": "staging",
		})
		assert.Equal(t, 2, deleted)

		// Verify only production metric remains
		metrics, _, _, _, _, _ := acc.Collect()
		assert.Equal(t, 1, len(metrics))
		assert.Equal(t, "metric2", metrics[0].Name())
	})

	// Reset for next test
	acc = newAccumulator(logger, time.Minute*5).(*lastValueAccumulator)
	acc.Accumulate(rm1)
	acc.Accumulate(rm2)
	acc.Accumulate(rm3)

	t.Run("CleanByResourceAttribute_Cluster", func(t *testing.T) {
		// Clean metrics with k8s.cluster.name="test-cluster"
		deleted := acc.CleanByLabels(map[string]string{
			"k8s.cluster.name": "test-cluster",
		})
		assert.Equal(t, 2, deleted)

		// Verify only prod-cluster metric remains
		metrics, _, _, _, _, _ := acc.Collect()
		assert.Equal(t, 1, len(metrics))
		assert.Equal(t, "metric2", metrics[0].Name())
	})

	// Reset for next test
	acc = newAccumulator(logger, time.Minute*5).(*lastValueAccumulator)
	acc.Accumulate(rm1)
	acc.Accumulate(rm2)
	acc.Accumulate(rm3)

	t.Run("CleanByMultipleResourceAttributes", func(t *testing.T) {
		// Clean metrics with both environment AND cluster
		deleted := acc.CleanByLabels(map[string]string{
			"deployment.environment": "staging",
			"k8s.cluster.name":       "test-cluster",
		})
		assert.Equal(t, 2, deleted) // Both rm1 and rm3 match

		// Verify only production metric remains
		metrics, _, _, _, _, _ := acc.Collect()
		assert.Equal(t, 1, len(metrics))
		assert.Equal(t, "metric2", metrics[0].Name())
	})
}

// Helper function to create test resource metrics
func createTestResourceMetrics(metricName, job, instance string, attributes map[string]interface{}) pmetric.ResourceMetrics {
	rm := pmetric.NewResourceMetrics()

	// Set resource attributes
	resource := rm.Resource()
	resource.Attributes().PutStr(string(conventions.ServiceNameKey), job)
	resource.Attributes().PutStr(string(conventions.ServiceInstanceIDKey), instance)

	// Create scope metrics
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("test-scope")

	// Create metric
	metric := sm.Metrics().AppendEmpty()
	metric.SetName(metricName)
	metric.SetDescription("Test metric")

	// Create gauge data point
	gauge := metric.SetEmptyGauge()
	dp := gauge.DataPoints().AppendEmpty()
	dp.SetDoubleValue(42.0)
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	// Set metric attributes
	for k, v := range attributes {
		dp.Attributes().PutStr(k, v.(string))
	}

	return rm
}

// Helper function to create test resource metrics with comprehensive resource attributes
func createTestResourceMetricsWithResourceAttrs(metricName string, resourceAttrs map[string]interface{}, dpAttrs map[string]interface{}) pmetric.ResourceMetrics {
	rm := pmetric.NewResourceMetrics()

	// Set resource attributes
	resource := rm.Resource()
	for k, v := range resourceAttrs {
		resource.Attributes().PutStr(k, v.(string))
	}

	// Create scope metrics
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("test-scope")

	// Create metric
	metric := sm.Metrics().AppendEmpty()
	metric.SetName(metricName)
	metric.SetDescription("Test metric")

	// Create gauge data point
	gauge := metric.SetEmptyGauge()
	dp := gauge.DataPoints().AppendEmpty()
	dp.SetDoubleValue(42.0)
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	// Set metric attributes
	for k, v := range dpAttrs {
		dp.Attributes().PutStr(k, v.(string))
	}

	return rm
}
