# Metrics Aggregator Processor

The Metrics Aggregator Processor aggregates metrics across different resources (e.g., across multiple pods in a Kubernetes cluster) to provide cluster-level or higher-level aggregated metrics.

## Configuration

The processor accepts the following configuration:

```yaml
processors:
  metricsaggregator:
    # Global configuration - applies to all aggregation rules
    group_by_labels:                            # Optional: Labels to group by for all rules
      - "agent_version"
      - "path_key"
    output_resource_attributes:                 # Required: Resource attributes for aggregated metrics
      otel_output_metric: "true"
      otel_output_processor: "metricsaggregator"
    aggregation_rules:
      - metric_pattern: "throughput"            # Pattern to match metric names
        match_type: "strict"                    # "strict" or "regex"
        output_metric_name: "cluster_throughput" # Name for the aggregated metric
        aggregation_type: "sum"                 # sum, mean, min, max, count
        preserve_original_metrics: false        # Whether to keep original metrics
        output_metric_type: "gauge"            # Output type: gauge, sum, histogram
```

### Configuration Fields

- `group_by_labels`: Array of label names to group by when aggregating (applies to all rules)
- `output_resource_attributes`: Map of resource attributes to add to all aggregated metrics (required)
- `aggregation_rules`: Array of aggregation rules to apply
  - `metric_pattern`: Pattern to match metric names (required)
  - `match_type`: How to match the pattern - "strict" (exact match) or "regex" (regular expression)
  - `output_metric_name`: Name for the aggregated metric (required)
  - `aggregation_type`: How to aggregate values - "sum", "mean", "min", "max", "count"
  - `preserve_original_metrics`: Whether to keep the original metrics (default: false)
  - `output_metric_type`: Type of the output metric - "gauge", "sum", "histogram"

## Examples

### Basic Sum Aggregation

```yaml
processors:
  metricsaggregator:
    group_by_labels:
      - "service_name"
    output_resource_attributes:
      otel_output_metric: "true"
    aggregation_rules:
      - metric_pattern: "http_requests_total"
        match_type: "strict"
        output_metric_name: "cluster_http_requests_total"
        aggregation_type: "sum"
        preserve_original_metrics: false
```

### Regex Pattern with Grouping

```yaml
processors:
  metricsaggregator:
    group_by_labels:
      - "service_name"
      - "endpoint"
    output_resource_attributes:
      aggregation_level: "cluster"
      aggregation_type: "latency_mean"
    aggregation_rules:
      - metric_pattern: ".*_latency_p50$"
        match_type: "regex"
        output_metric_name: "cluster_latency_p50_mean"
        aggregation_type: "mean"
```

### Multiple Rules

```yaml
processors:
  metricsaggregator:
    group_by_labels:
      - "agent_version"
      - "path_key"
    output_resource_attributes:
      otel_output_metric: "true"
      otel_output_processor: "metricsaggregator"
    aggregation_rules:
      # Aggregate throughput metrics
      - metric_pattern: "throughput"
        match_type: "strict"
        output_metric_name: "cluster_throughput"
        aggregation_type: "sum"
        preserve_original_metrics: false
        
      # Aggregate latency metrics
      - metric_pattern: ".*latency.*"
        match_type: "regex"
        output_metric_name: "cluster_latency_mean"
        aggregation_type: "mean"
        preserve_original_metrics: false
```

## How It Works

1. **Collection**: The processor collects all metrics that match the specified patterns
2. **Grouping**: Metrics are grouped by the global `group_by_labels` values
3. **Aggregation**: Values within each group are aggregated using the specified aggregation type
4. **Output**: New aggregated metrics are created with the specified output name and type
5. **Cleanup**: If `preserve_original_metrics` is false, original matching metrics are removed

## Aggregation Types

- **sum**: Add up all values
- **mean**: Calculate the average of all values
- **min**: Take the minimum value
- **max**: Take the maximum value
- **count**: Count the number of data points

## Output Metric Types

- **gauge**: Point-in-time value (default)
- **sum**: Cumulative value
- **histogram**: Simple histogram with sum and count

## Use Cases

- **Cluster-level monitoring**: Aggregate pod-level metrics to cluster-level metrics
- **Service-level aggregation**: Combine metrics from multiple instances of a service
- **Cross-resource analysis**: Sum throughput, average latency across multiple resources
- **Cost optimization**: Reduce the number of individual metrics while preserving aggregate insights

## Performance Considerations

- The processor processes rules sequentially
- Large numbers of metrics or complex regex patterns may impact performance
- Consider using specific patterns rather than broad regex matches when possible 