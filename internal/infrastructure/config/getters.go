package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// getEnvStr returns the value of key, or defaultValue when unset or empty.
func getEnvStr(key, defaultValue string) (string, error) {
	if v := os.Getenv(key); v != "" {
		return v, nil
	}
	return defaultValue, nil
}

// getEnvInt parses key as an int. Returns defaultValue when unset.
//
// A malformed value is a fatal configuration error: silently falling back
// would let a typo like SERVER_PORT=80O go unnoticed.
func getEnvInt(key string, defaultValue int) (int, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return defaultValue, nil
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf(
			"%s must be an integer: %w",
			key,
			err,
		)
	}

	return value, nil
}

// getEnvFloat64 parses key as a float64. Returns defaultValue when unset.
func getEnvFloat64(key string, defaultValue float64) (float64, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return defaultValue, nil
	}

	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf(
			"%s must be a float64: %w",
			key,
			err,
		)
	}
	return v, nil
}

// getEnvDuration parses key as a Go duration string (e.g. "10s", "24h").
// Returns defaultValue when unset.
func getEnvDuration(key string, defaultValue time.Duration) (time.Duration, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return defaultValue, nil
	}

	v, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf(
			"%s must be a duration: %w",
			key,
			err,
		)
	}
	return v, nil
}

// getEnvBool parses key as a boolean. Accepts the values understood by
// strconv.ParseBool (1, t, T, TRUE, true, True, 0, f, F, FALSE, false, False).
func getEnvBool(key string, defaultValue bool) (bool, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return defaultValue, nil
	}

	v, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf(
			"%s must be a boolean: %w",
			key,
			err,
		)
	}
	return v, nil
}

// getEnvBase64 decodes key as standard base64. Returns nil when unset.
//
// Used for cryptographic secrets that must be transferred as text.
func getEnvBase64(key string) ([]byte, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return nil, nil
	}

	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf(
			"%s must be valid base64: %w",
			key,
			err,
		)
	}
	return decoded, nil
}

// getEnvCSV splits key by commas, trimming whitespace. Returns defaultValue
// when unset or when every element is empty.
func getEnvCSV(key string, defaultValue []string) ([]string, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return defaultValue, nil
	}

	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))

	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}

	if len(out) == 0 {
		return defaultValue, nil
	}
	return out, nil
}

// getEnvMap parses "k1=v1,k2=v2" pairs. Returns nil when unset.
//
// Used for OTEL_EXPORTER_OTLP_HEADERS, which follows this convention.
func getEnvMap(key string) (map[string]string, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return nil, nil
	}

	out := make(map[string]string)
	for _, pair := range strings.Split(raw, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(pair), "=")
		if !ok || k == "" {
			continue
		}
		out[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// redacted is used when logging secret values. Currently unused in this
// file but kept for callers that need consistent redaction.
//
//nolint:unused
func redacted(s string) string {
	if s == "" {
		return ""
	}
	return fmt.Sprintf("<redacted:%d bytes>", len(s))
}
