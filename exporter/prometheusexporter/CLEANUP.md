# Prometheus Exporter Metric Cleanup

The Prometheus exporter now supports dynamic metric cleanup through both programmatic interfaces and HTTP API endpoints.

## 🎯 **Overview**

The cleanup functionality allows you to selectively remove metrics from the exporter's internal storage based on:

- **Label filters**: Remove metrics matching specific label key-value pairs
- **Metric name patterns**: Remove metrics by name using string matching or regex patterns  
- **Expiration**: Manually trigger removal of expired metrics

## 🏗️ **Architecture**

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│  External       │    │  Cleanup API     │    │  Prometheus     │
│  Signal/API     │───▶│  (HTTP)          │───▶│  Exporter       │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                                                         │
                                                         ▼
                                               ┌─────────────────┐
                                               │  Collector      │
                                               └─────────────────┘
                                                         │
                                                         ▼
                                               ┌─────────────────┐
                                               │  Accumulator    │
                                               │  (sync.Map)     │
                                               └─────────────────┘
```

## 🚀 **Usage**

### HTTP API Endpoints

The exporter automatically exposes cleanup endpoints alongside the standard `/metrics` endpoint:

- `POST /cleanup` - Execute cleanup operations
- `GET /cleanup/status` - Get API status and examples  
- `GET /cleanup/metrics` - Get current metric count

### Cleanup by Labels

Remove metrics matching specific label values:

```bash
curl -X POST http://localhost:8888/cleanup \
  -H "Content-Type: application/json" \
  -d '{
    "type": "labels",
    "filters": {
      "job": "test-service",
      "environment": "staging"
    }
  }'
```

**Response:**
```json
{
  "success": true,
  "deleted_count": 15,
  "message": "Successfully deleted 15 metrics",
  "timestamp": "2023-12-07T10:30:45Z"
}
```

### Cleanup by Metric Name

Remove metrics by name pattern (supports regex):

```bash
curl -X POST http://localhost:8888/cleanup \
  -H "Content-Type: application/json" \
  -d '{
    "type": "name", 
    "pattern": "temp_metric_.*"
  }'
```

### Cleanup Expired Metrics

Manually trigger expiration cleanup:

```bash
curl -X POST http://localhost:8888/cleanup \
  -H "Content-Type: application/json" \
  -d '{
    "type": "expired"
  }'
```

### API Status

Get information about available operations:

```bash
curl http://localhost:8888/cleanup/status
```

**Response:**
```json
{
  "cleanup_api_version": "1.0",
  "supported_operations": ["labels", "name", "expired"],
  "endpoints": {
    "cleanup": "/cleanup",
    "status": "/cleanup/status"
  },
  "examples": {
    "cleanup_by_labels": {
      "type": "labels",
      "filters": {
        "job": "test-job",
        "instance": "test-instance"
      }
    }
  }
}
```

## 🔧 **Programmatic Usage**

If you have access to the exporter instance, you can call cleanup methods directly:

```go
// Cleanup by labels
deletedCount := exporter.CleanByLabels(map[string]string{
    "job": "test-service",
    "environment": "staging",
})

// Cleanup by name pattern
deletedCount := exporter.CleanByMetricName("temp_.*")

// Cleanup expired metrics
deletedCount := exporter.CleanExpired()
```

## 📋 **Supported Label Filters**

The cleanup functionality can filter on **business-relevant labels** present in your metrics:

- **✅ All Resource Attributes**: Any resource attribute (e.g., `service.name`, `service.version`, `deployment.environment`, `k8s.cluster.name`, `cloud.provider`, etc.)
- **✅ Standard Prometheus Labels**: `job`, `instance` (derived from service.name/service.instance.id)
- **✅ Metric Attributes**: Any labels attached to individual data points
- **❌ Scope Attributes**: Instrumentation library metadata (`otel_scope_*`) is **not included** in cleanup filtering as it's rarely used for metric management

### **Resource-to-Telemetry Conversion Support**

🔥 **Important**: When `resource_to_telemetry_conversion.enabled: true` is configured, **ALL resource attributes get converted to metric labels**. Our cleanup functionality **fully supports** this by extracting all resource attributes for filtering.

**Example Configuration:**
```yaml
exporters:
  prometheus:
    endpoint: "0.0.0.0:8888"
    enable_cleanup_api: true
    resource_to_telemetry_conversion:
      enabled: true  # Converts ALL resource attributes to metric labels
```

**Example Cleanup:**
```bash
# Clean metrics by any resource attribute
curl -X POST http://localhost:8888/cleanup \
  -H "Content-Type: application/json" \
  -d '{
    "type": "labels",
    "filters": {
      "k8s.cluster.name": "staging-cluster",
      "deployment.environment": "staging",
      "cloud.region": "us-west-2"
    }
  }'
```

## ⚠️ **Important Notes**

1. **Thread Safety**: All cleanup operations are thread-safe and can be called concurrently with metric ingestion
2. **Immediate Effect**: Cleaned metrics are immediately removed from `/metrics` endpoint responses
3. **No Recovery**: Deleted metrics cannot be recovered unless new data points arrive
4. **Performance**: Cleanup operations iterate through all stored metrics, so use judiciously on high-cardinality setups

## 🎛️ **Configuration**

**⚠️ IMPORTANT**: The cleanup API is **disabled by default** for security reasons. You must explicitly enable it in your configuration:

```yaml
exporters:
  prometheus:
    endpoint: "0.0.0.0:8888"
    enable_cleanup_api: true  # REQUIRED: Enable cleanup API endpoints
```

### Configuration Options

| Option | Default | Description |
|--------|---------|-------------|
| `enable_cleanup_api` | `false` | Enables cleanup API endpoints (`/cleanup`, `/cleanup/status`, `/cleanup/metrics`) |

### Security Considerations

- **Production Safety**: Cleanup API is disabled by default to prevent accidental metric deletion
- **Access Control**: Consider implementing additional authentication/authorization if enabling in production
- **Network Security**: Ensure proper firewall rules if exposing the cleanup endpoints

The existing `metric_expiration` setting still controls automatic expiration behavior.

## 🔍 **Monitoring Cleanup Operations**

Cleanup operations are logged at INFO level:

```
INFO	Cleaned metrics by labels	{"deleted_count": 15, "filters": {"job":"test-service"}}
INFO	Cleaned metrics by name pattern	{"deleted_count": 8, "pattern": "temp_.*"}
INFO	Cleaned expired metrics	{"deleted_count": 23}
```

## 📊 **Examples**

### Remove All Metrics from a Specific Service

```bash
curl -X POST http://localhost:8888/cleanup \
  -H "Content-Type: application/json" \
  -d '{
    "type": "labels",
    "filters": {
      "service.name": "payment-service"
    }
  }'
```

### Remove Test/Debug Metrics

```bash
curl -X POST http://localhost:8888/cleanup \
  -H "Content-Type: application/json" \
  -d '{
    "type": "name",
    "pattern": "(test_|debug_|temp_).*"
  }'
```

### Remove Metrics from Specific Environment and Service

```bash
curl -X POST http://localhost:8888/cleanup \
  -H "Content-Type: application/json" \
  -d '{
    "type": "labels", 
    "filters": {
      "deployment.environment": "staging",
      "service.name": "payment-service",
      "service.version": "v1.0.0"
    }
  }'
```

## 🧪 **Testing**

The implementation includes comprehensive tests covering:

- Label-based filtering accuracy
- Name pattern matching (both string and regex)
- Thread safety of cleanup operations
- HTTP API functionality
- Error handling

Run tests with:
```bash
go test -v ./exporter/prometheusexporter/... -run TestCleanup
``` 