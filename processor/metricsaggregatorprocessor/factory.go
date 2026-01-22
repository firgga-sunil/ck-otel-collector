// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricsaggregatorprocessor

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

const (
	// typeStr is the type of the processor
	typeStr = "metricsaggregator"
	// The value of "type" key in configuration.
	stability = component.StabilityLevelDevelopment
)

// NewFactory creates a new processor factory
func NewFactory() processor.Factory {
	return processor.NewFactory(
		component.MustNewType(typeStr),
		createDefaultConfig,
		processor.WithMetrics(createMetricsProcessor, stability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		AggregationRules: []AggregationRule{},
	}
}

func createMetricsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (processor.Metrics, error) {
	processorConfig := cfg.(*Config)
	return processorhelper.NewMetrics(
		ctx,
		set,
		cfg,
		nextConsumer,
		newMetricsAggregatorProcessor(processorConfig, set.Logger).processMetrics,
		processorhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}),
		processorhelper.WithStart(func(context.Context, component.Host) error { return nil }),
		processorhelper.WithShutdown(func(context.Context) error { return nil }),
	)
}
