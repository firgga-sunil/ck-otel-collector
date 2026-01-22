// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/ck-otel-collector/receiver/otlpreceiver/internal/metrics"

import (
	"context"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"github.com/ck-otel-collector/receiver/otlpreceiver/internal/errors"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"google.golang.org/grpc/metadata"
)

const dataFormatProtobuf = "protobuf"

// HeaderMapping defines how to map a header to an attribute
type HeaderMapping struct {
	HeaderName    string
	AttributeName string
}

// HeaderExtractionConfig defines configuration for header extraction
type HeaderExtractionConfig struct {
	Enabled          bool
	HeadersToExtract []HeaderMapping
}

// Receiver is the type used to handle metrics from OpenTelemetry exporters.
type Receiver struct {
	pmetricotlp.UnimplementedGRPCServer
	nextConsumer consumer.Metrics
	obsreport    *receiverhelper.ObsReport
	headerConfig HeaderExtractionConfig
}

// New creates a new Receiver reference.
func New(nextConsumer consumer.Metrics, obsreport *receiverhelper.ObsReport) *Receiver {
	return &Receiver{
		nextConsumer: nextConsumer,
		obsreport:    obsreport,
	}
}

// NewWithHeaderExtraction creates a new Receiver reference with header extraction capabilities.
func NewWithHeaderExtraction(nextConsumer consumer.Metrics, obsreport *receiverhelper.ObsReport, headerConfig HeaderExtractionConfig) *Receiver {
	return &Receiver{
		nextConsumer: nextConsumer,
		obsreport:    obsreport,
		headerConfig: headerConfig,
	}
}

// extractHeadersToAttributes extracts headers from gRPC context and adds them as resource attributes
func (r *Receiver) extractHeadersToAttributes(ctx context.Context, md pmetric.Metrics) {
	if !r.headerConfig.Enabled {
		return
	}

	// Extract headers from gRPC context
	grpcMD, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return
	}

	// Process each resource
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)

		// Add headers as resource attributes
		for _, mapping := range r.headerConfig.HeadersToExtract {
			if values := grpcMD.Get(mapping.HeaderName); len(values) > 0 {
				rm.Resource().Attributes().PutStr(mapping.AttributeName, values[0])
			}
		}
	}
}

// Export implements the service Export metrics func.
func (r *Receiver) Export(ctx context.Context, req pmetricotlp.ExportRequest) (pmetricotlp.ExportResponse, error) {
	md := req.Metrics()
	dataPointCount := md.DataPointCount()
	if dataPointCount == 0 {
		return pmetricotlp.NewExportResponse(), nil
	}

	// Extract headers and add as attributes if enabled
	r.extractHeadersToAttributes(ctx, md)

	ctx = r.obsreport.StartMetricsOp(ctx)
	err := r.nextConsumer.ConsumeMetrics(ctx, md)
	r.obsreport.EndMetricsOp(ctx, dataFormatProtobuf, dataPointCount, err)

	// Use appropriate status codes for permanent/non-permanent errors
	// If we return the error straightaway, then the grpc implementation will set status code to Unknown
	// Refer: https://github.com/grpc/grpc-go/blob/v1.59.0/server.go#L1345
	// So, convert the error to appropriate grpc status and return the error
	// NonPermanent errors will be converted to codes.Unavailable (equivalent to HTTP 503)
	// Permanent errors will be converted to codes.InvalidArgument (equivalent to HTTP 400)
	if err != nil {
		return pmetricotlp.NewExportResponse(), errors.GetStatusFromError(err)
	}

	return pmetricotlp.NewExportResponse(), nil
}
