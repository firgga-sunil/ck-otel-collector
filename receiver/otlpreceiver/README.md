# OTLP Receiver

The OTLP (OpenTelemetry Protocol) receiver is a component that receives telemetry data from applications and services using the OpenTelemetry Protocol.

## Features

- **OTLP gRPC Support**: Receives telemetry data via OTLP gRPC protocol
- **OTLP HTTP Support**: Receives telemetry data via OTLP HTTP protocol
- **Header Extraction**: Extracts headers from gRPC requests and adds them as attributes to metrics
- **Multi-signal Support**: Supports traces, metrics, logs, and profiles

## Header Extraction Feature

The OTLP receiver includes a powerful header extraction feature that allows you to extract headers from incoming gRPC requests and add them as attributes to your telemetry data. This is particularly useful for:

- **Multi-tenancy**: Extract tenant IDs from headers
- **Environment tagging**: Add environment information to metrics
- **Service identification**: Tag metrics with service names
- **Custom routing**: Use headers for custom metric routing

### Configuration

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: "0.0.0.0:4317"
    header_extraction:
      enabled: true
      headers_to_extract:
        # Extract tenant ID as resource attribute
        - header_name: "x-tenant-id"
          attribute_name: "tenant_id"
          resource_attribute: true
        # Extract environment as metric attribute
        - header_name: "x-environment"
          attribute_name: "environment"
          resource_attribute: false
        # Extract service name as resource attribute
        - header_name: "x-service-name"
          attribute_name: "service_name"
          resource_attribute: true
```

### Header Mapping Configuration

Each header mapping has the following fields:

- **`header_name`**: The name of the header to extract (e.g., "x-tenant-id")
- **`attribute_name`**: The name of the attribute to set in the telemetry data

**Note**: All extracted headers are automatically added as resource attributes.

### Usage Examples

#### Example 1: Multi-tenant Application

```yaml
header_extraction:
  enabled: true
  headers_to_extract:
    - header_name: "x-tenant-id"
      attribute_name: "tenant_id"
    - header_name: "x-environment"
      attribute_name: "environment"
```

This configuration will:
- Extract `x-tenant-id` header and add it as `tenant_id` resource attribute
- Extract `x-environment` header and add it as `environment` resource attribute

#### Example 2: Service-specific Metrics

```yaml
header_extraction:
  enabled: true
  headers_to_extract:
    - header_name: "x-service-name"
      attribute_name: "service_name"
    - header_name: "x-instance-id"
      attribute_name: "instance_id"
```

This configuration will:
- Extract `x-service-name` header and add it as `service_name` resource attribute
- Extract `x-instance-id` header and add it as `instance_id` resource attribute

### Integration with Metrics Aggregator

The header extraction feature works seamlessly with the metrics aggregator processor. When headers are extracted as resource attributes, they will be available for grouping in the metrics aggregator:

```yaml
processors:
  metricsaggregator:
    group_by_labels:
      - "tenant_id"      # From header extraction (resource attribute)
      - "environment"    # From header extraction (resource attribute)
      - "service_name"   # From header extraction (resource attribute)
```

### Client Usage

To use header extraction, clients should include the configured headers in their gRPC requests:

```bash
# Example with curl (for HTTP)
curl -X POST http://localhost:4318/v1/metrics \
  -H "Content-Type: application/x-protobuf" \
  -H "x-tenant-id: tenant-123" \
  -H "x-environment: production" \
  -H "x-service-name: user-service" \
  --data-binary @metrics.pb
```

```go
// Example with Go gRPC client
import (
    "google.golang.org/grpc/metadata"
    "context"
)

func sendMetrics() {
    ctx := context.Background()
    md := metadata.New(map[string]string{
        "x-tenant-id": "tenant-123",
        "x-environment": "production",
        "x-service-name": "user-service",
    })
    ctx = metadata.NewOutgoingContext(ctx, md)
    
    // Send metrics with headers
    client.Export(ctx, request)
}
```

### Validation

The receiver validates header extraction configuration:

- If `enabled` is `true`, at least one header mapping must be specified
- Each header mapping must have non-empty `header_name` and `attribute_name`
- Invalid configurations will cause the receiver to fail to start

### Performance Considerations

- Header extraction adds minimal overhead to metric processing
- Headers are extracted once per request and applied to all metrics in that request
- Resource attributes are more efficient than metric attributes for high-cardinality scenarios

## Getting Started

1. **Configure the receiver** with header extraction settings
2. **Deploy the collector** with the custom OTLP receiver
3. **Send metrics** with the appropriate headers from your applications
4. **Verify** that headers appear as attributes in your metrics

## Troubleshooting

### Common Issues

1. **Headers not appearing**: Ensure header names match exactly (case-sensitive)
2. **Configuration errors**: Check that all required fields are specified
3. **Performance issues**: Consider using resource attributes instead of metric attributes for high-volume scenarios

### Debugging

Enable debug logging to see header extraction in action:

```yaml
config:
  receivers:
    otlp:
      protocols:
        grpc:
          endpoint: "0.0.0.0:4317"
      header_extraction:
        enabled: true
        headers_to_extract:
          - header_name: "x-tenant-id"
            attribute_name: "tenant_id"
            resource_attribute: true
  exporters:
    debug:
      verbosity: detailed
```

This will show detailed information about header extraction in the collector logs.
