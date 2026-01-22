// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter"

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type prometheusExporter struct {
	config       Config
	name         string
	endpoint     string
	shutdownFunc func(ctx context.Context) error
	handler      http.Handler
	collector    *collector
	registry     *prometheus.Registry
	settings     component.TelemetrySettings
}

var errBlankPrometheusAddress = errors.New("expecting a non-blank address to run the Prometheus metrics handler")

func newPrometheusExporter(config *Config, set exporter.Settings) (*prometheusExporter, error) {
	addr := strings.TrimSpace(config.Endpoint)
	if strings.TrimSpace(config.Endpoint) == "" {
		return nil, errBlankPrometheusAddress
	}

	collector := newCollector(config, set.Logger)
	registry := prometheus.NewRegistry()
	_ = registry.Register(collector)
	return &prometheusExporter{
		config:       *config,
		name:         set.ID.String(),
		endpoint:     addr,
		collector:    collector,
		registry:     registry,
		shutdownFunc: func(_ context.Context) error { return nil },
		handler: promhttp.HandlerFor(
			registry,
			promhttp.HandlerOpts{
				ErrorHandling:     promhttp.ContinueOnError,
				ErrorLog:          newPromLogger(set.Logger),
				EnableOpenMetrics: config.EnableOpenMetrics,
			},
		),
		settings: set.TelemetrySettings,
	}, nil
}

func (pe *prometheusExporter) Start(ctx context.Context, host component.Host) error {
	ln, err := pe.config.ToListener(ctx)
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", pe.handler)

	// ========== ENHANCEMENT: Cleanup API Endpoints ==========
	// Register cleanup API endpoints only if enabled in configuration
	if pe.config.EnableCleanupAPI {
		cleanupAPI := NewCleanupAPI(pe, pe.settings.Logger)
		// HandleFunc is used instead of Handle because our cleanup handlers are functions,
		// not types implementing http.Handler interface. HandleFunc converts function to Handler.
		mux.HandleFunc("/cleanup", cleanupAPI.CleanupHandler)
		mux.HandleFunc("/cleanup/status", cleanupAPI.StatusHandler)
		mux.HandleFunc("/cleanup/metrics", cleanupAPI.MetricsHandler)
		pe.settings.Logger.Info("Cleanup API endpoints enabled",
			zap.String("endpoints", "/cleanup, /cleanup/status, /cleanup/metrics"))
	}
	// =========================================================

	// ========== ENHANCEMENT: Web UI Endpoints ==========
	// Register web UI endpoints
	webUI := NewWebUI(pe.settings.Logger)
	mux.HandleFunc("/", webUI.IndexHandler)
	mux.HandleFunc("/ui", webUI.IndexHandler)
	mux.HandleFunc("/static/", webUI.StaticHandler)
	pe.settings.Logger.Info("Web UI endpoints enabled",
		zap.String("endpoints", "/, /ui, /static/"))
	// ===================================================

	srv, err := pe.config.ToServer(ctx, host, pe.settings, mux)
	if err != nil {
		lnerr := ln.Close()
		return errors.Join(err, lnerr)
	}
	pe.shutdownFunc = func(ctx context.Context) error {
		return srv.Shutdown(ctx)
	}
	go func() {
		_ = srv.Serve(ln)
	}()

	return nil
}

func (pe *prometheusExporter) ConsumeMetrics(_ context.Context, md pmetric.Metrics) error {
	n := 0
	rmetrics := md.ResourceMetrics()
	for i := 0; i < rmetrics.Len(); i++ {
		n += pe.collector.processMetrics(rmetrics.At(i))
	}

	return nil
}

func (pe *prometheusExporter) Shutdown(ctx context.Context) error {
	return pe.shutdownFunc(ctx)
}

// ========== ENHANCEMENT: Metric Cleanup Methods ==========

// CleanByLabels removes metrics based on label filters
func (pe *prometheusExporter) CleanByLabels(filters map[string]string) int {
	return pe.collector.CleanByLabels(filters)
}

// CleanByMetricName removes metrics matching name pattern
func (pe *prometheusExporter) CleanByMetricName(namePattern string) int {
	return pe.collector.CleanByMetricName(namePattern)
}

// CleanExpired removes expired metrics
func (pe *prometheusExporter) CleanExpired() int {
	return pe.collector.CleanExpired()
}

// ================================================================
