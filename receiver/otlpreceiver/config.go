// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otlpreceiver // import "go.opentelemetry.io/collector/receiver/otlpreceiver"

import (
	"encoding"
	"errors"
	"fmt"
	"net/url"
	"path"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configoptional"
)

type SanitizedURLPath string

var _ encoding.TextUnmarshaler = (*SanitizedURLPath)(nil)

func (s *SanitizedURLPath) UnmarshalText(text []byte) error {
	u, err := url.Parse(string(text))
	if err != nil {
		return fmt.Errorf("invalid HTTP URL path set for signal: %w", err)
	}

	if !path.IsAbs(u.Path) {
		u.Path = "/" + u.Path
	}

	*s = SanitizedURLPath(u.Path)
	return nil
}

// HeaderMapping defines how to map a header to an attribute
type HeaderMapping struct {
	// HeaderName is the name of the header to extract
	HeaderName string `mapstructure:"header_name"`
	// AttributeName is the name of the attribute to set
	AttributeName string `mapstructure:"attribute_name"`
}

// HeaderExtractionConfig defines configuration for header extraction
type HeaderExtractionConfig struct {
	// Enabled enables header extraction
	Enabled bool `mapstructure:"enabled"`
	// HeadersToExtract defines which headers to extract and how to map them
	HeadersToExtract []HeaderMapping `mapstructure:"headers_to_extract"`
}

type HTTPConfig struct {
	ServerConfig confighttp.ServerConfig `mapstructure:",squash"`

	// The URL path to receive traces on. If omitted "/v1/traces" will be used.
	TracesURLPath SanitizedURLPath `mapstructure:"traces_url_path,omitempty"`

	// The URL path to receive metrics on. If omitted "/v1/metrics" will be used.
	MetricsURLPath SanitizedURLPath `mapstructure:"metrics_url_path,omitempty"`

	// The URL path to receive logs on. If omitted "/v1/logs" will be used.
	LogsURLPath SanitizedURLPath `mapstructure:"logs_url_path,omitempty"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// Protocols is the configuration for the supported protocols.
type Protocols struct {
	GRPC configoptional.Optional[configgrpc.ServerConfig] `mapstructure:"grpc"`
	HTTP configoptional.Optional[HTTPConfig]              `mapstructure:"http"`
	// prevent unkeyed literal initialization
	_ struct{}
}

// Config defines configuration for OTLP receiver.
type Config struct {
	// Protocols is the configuration for the supported protocols, currently gRPC and HTTP (Proto and JSON).
	Protocols `mapstructure:"protocols"`
	// HeaderExtraction defines configuration for extracting headers and adding them as attributes
	HeaderExtraction HeaderExtractionConfig `mapstructure:"header_extraction"`
}

var _ component.Config = (*Config)(nil)

// Validate checks the receiver configuration is valid
func (cfg *Config) Validate() error {
	if !cfg.GRPC.HasValue() && !cfg.HTTP.HasValue() {
		return errors.New("must specify at least one protocol when using the OTLP receiver")
	}

	// Validate header extraction configuration
	if cfg.HeaderExtraction.Enabled {
		if len(cfg.HeaderExtraction.HeadersToExtract) == 0 {
			return errors.New("header_extraction.enabled is true but no headers_to_extract are specified")
		}

		for i, mapping := range cfg.HeaderExtraction.HeadersToExtract {
			if mapping.HeaderName == "" {
				return fmt.Errorf("header_extraction.headers_to_extract[%d].header_name cannot be empty", i)
			}
			if mapping.AttributeName == "" {
				return fmt.Errorf("header_extraction.headers_to_extract[%d].attribute_name cannot be empty", i)
			}
		}
	}

	return nil
}
