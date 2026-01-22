// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricsaggregatorprocessor

import (
	"errors"
	"fmt"
	"regexp"

	"go.opentelemetry.io/collector/component"
)

// Config represents the receiver configuration.
type Config struct {
	GroupByLabels            []string          `mapstructure:"group_by_labels"`
	OutputResourceAttributes map[string]string `mapstructure:"output_resource_attributes"`
	AggregationRules         []AggregationRule `mapstructure:"aggregation_rules"`
}

// AggregationRule defines how to aggregate metrics
type AggregationRule struct {
	MetricPattern           string `mapstructure:"metric_pattern"`
	MatchType               string `mapstructure:"match_type"`
	OutputMetricName        string `mapstructure:"output_metric_name"`
	AggregationType         string `mapstructure:"aggregation_type"`
	PreserveOriginalMetrics bool   `mapstructure:"preserve_original_metrics"`
	OutputMetricType        string `mapstructure:"output_metric_type"`
}

var _ component.Config = (*Config)(nil)

// Validate checks the receiver configuration is valid
func (cfg *Config) Validate() error {
	if len(cfg.GroupByLabels) == 0 {
		return errors.New("group_by_labels cannot be empty - at least one label must be specified for grouping")
	}

	if len(cfg.OutputResourceAttributes) == 0 {
		return errors.New("output_resource_attributes cannot be empty - required to distinguish aggregated metrics from original metrics")
	}

	if len(cfg.AggregationRules) == 0 {
		return errors.New("at least one aggregation rule must be specified")
	}

	for i, rule := range cfg.AggregationRules {
		if err := validateAggregationRule(rule, i); err != nil {
			return err
		}
	}

	return nil
}

func validateAggregationRule(rule AggregationRule, index int) error {
	if rule.MetricPattern == "" {
		return fmt.Errorf("aggregation rule %d: metric_pattern cannot be empty", index)
	}

	if rule.MatchType == "" {
		rule.MatchType = "strict" // default
	}

	validMatchTypes := map[string]bool{
		"strict": true,
		"regex":  true,
	}
	if !validMatchTypes[rule.MatchType] {
		return fmt.Errorf("aggregation rule %d: invalid match_type '%s', must be 'strict' or 'regex'", index, rule.MatchType)
	}

	// Validate regex pattern if match_type is regex
	if rule.MatchType == "regex" {
		if _, err := regexp.Compile(rule.MetricPattern); err != nil {
			return fmt.Errorf("aggregation rule %d: invalid regex pattern '%s': %w", index, rule.MetricPattern, err)
		}
	}

	if rule.OutputMetricName == "" {
		return fmt.Errorf("aggregation rule %d: output_metric_name cannot be empty", index)
	}

	validAggregationTypes := map[string]bool{
		"sum":   true,
		"mean":  true,
		"min":   true,
		"max":   true,
		"count": true,
	}
	if rule.AggregationType == "" {
		rule.AggregationType = "sum" // default
	}
	if !validAggregationTypes[rule.AggregationType] {
		return fmt.Errorf("aggregation rule %d: invalid aggregation_type '%s', must be one of: sum, mean, min, max, count", index, rule.AggregationType)
	}

	validOutputTypes := map[string]bool{
		"gauge":     true,
		"sum":       true,
		"histogram": true,
	}
	if rule.OutputMetricType != "" && !validOutputTypes[rule.OutputMetricType] {
		return fmt.Errorf("aggregation rule %d: invalid output_metric_type '%s', must be one of: gauge, sum, histogram", index, rule.OutputMetricType)
	}

	return nil
}
