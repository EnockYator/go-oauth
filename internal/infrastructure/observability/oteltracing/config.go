package oteltracing

import (
	"errors"
	"fmt"
	"time"
)

// Protocol selects the OTLP transport used to export spans.
type Protocol string

const (
	// ProtocolGRPC is the preferred transport for internal networks:
	// lower latency, streaming, HTTP/2 multiplexing.
	ProtocolGRPC Protocol = "grpc"

	// ProtocolHTTP is preferred when gRPC is blocked by firewalls or
	// proxies, or when exporting to SaaS backends that only expose HTTP.
	ProtocolHTTP Protocol = "http"
)

// Config describes everything tracing needs to initialize.
//
// Zero values are safe: the OTLP exporters fall back to standard
// OpenTelemetry environment variables (OTEL_EXPORTER_OTLP_*), and the
// batch span processor falls back to the OTel SDK defaults.
type Config struct {
	// ---- Service identity --------------------------------------------
	AppName       string
	AppVersion    string
	DeploymentEnv string

	// ---- Transport ---------------------------------------------------
	Protocol Protocol // Defaults to ProtocolGRPC.
	Endpoint string   // e.g. "otel-collector.observability.svc:4317"

	// ---- Security ----------------------------------------------------
	// Insecure disables TLS entirely. ONLY use for local development
	// or when the collector is on a trusted private network.
	Insecure bool

	// Optional client-side TLS material. If empty, the system trust
	// store is used.
	TLSCertFile   string
	TLSKeyFile    string
	TLSCAFile     string
	TLSServerName string // Overrides SNI / verification hostname.

	// Headers are sent on every export request. Use for backends that
	// require an API key (Honeycomb, Grafana Cloud, Lightstep, ...).
	Headers map[string]string

	// Compression: "" (none) or "gzip".
	Compression string

	// ---- Timing ------------------------------------------------------
	ExportTimeout   time.Duration // Per-export request timeout.
	ShutdownTimeout time.Duration // Reserved for the caller's shutdown ctx.

	// ---- Sampling ----------------------------------------------------
	// SamplingRatio is the TraceIDRatioBased sampling probability for
	// root spans. Child spans follow the parent's decision.
	SamplingRatio float64

	// ---- Batch span processor tuning --------------------------------
	// All optional. Zero values fall back to the SDK defaults.
	BatchTimeout       time.Duration
	MaxExportBatchSize int
	MaxQueueSize       int
}

// normalize fills in defaults and validates the configuration.
func (c *Config) normalize() error {
	if c.AppName == "" {
		return errors.New("tracing: service name is required")
	}

	if c.SamplingRatio < 0 || c.SamplingRatio > 1 {
		return fmt.Errorf(
			"tracing: sampling ratio must be between 0 and 1, got %v",
			c.SamplingRatio,
		)
	}

	if c.Protocol == "" {
		c.Protocol = ProtocolGRPC
	}

	switch c.Protocol {
	case ProtocolGRPC, ProtocolHTTP:
		// ok
	default:
		return fmt.Errorf("tracing: unsupported protocol %q", c.Protocol)
	}

	switch c.Compression {
	case "", "gzip":
		// ok
	default:
		return fmt.Errorf(
			"tracing: unsupported compression %q (want \"\" or \"gzip\")",
			c.Compression,
		)
	}

	if c.Insecure && (c.TLSCertFile != "" || c.TLSCAFile != "") {
		return errors.New(
			"tracing: Insecure cannot be combined with TLS files",
		)
	}

	if c.ExportTimeout <= 0 {
		c.ExportTimeout = 10 * time.Second
	}
	if c.ShutdownTimeout <= 0 {
		c.ShutdownTimeout = 5 * time.Second
	}

	return nil
}
