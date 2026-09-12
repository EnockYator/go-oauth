package config

import (
	"log/slog"
	"os"
	"strconv"
	"time"
)

// getEnvStr gets the string value of the evironment variable that matches the key.
// If the string is empty, it returns the default value passed.
func getEnvStr(key, defaultValue string) (string)  {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt gets the interger value of the evironment variable that matches the key.
// If the value is 0, it returns the default value passed.
func getEnvInt(key string, defaultValue int) (int)  {
	if value := os.Getenv(key); value != "" {
		i, err := strconv.Atoi(value)
		if err != nil {
			slog.Error(
				"failed converting string env value to interger",
				slog.Any("env_value", err),
			)
			os.Exit(1)
		}
		return i
	}
	return defaultValue
}

// getEnvDuration gets the duration value of the evironment variable that matches the key
// If the duration is not set, it returns the default value passed.
func getEnvDuration(key string, defaultValue time.Duration) (time.Duration)  {
	if value := os.Getenv(key); value != "" {
		d, err := time.ParseDuration(value)
		if err != nil {
			slog.Error("failed to convert env string to duration",
				slog.Any("env_value", err),
			)
			os.Exit(1)
		}
		return d
	}
	return  defaultValue
}