package config

import (
	"time"
)

// Load loads all the configurations required to start the application
func Load() (*Config, error) {
	cfg := &Config{
		App: App{
			AppEnv:  getEnvStr("APP_ENV", "production"),
			AppName: getEnvStr("APP_NAME", ""),
		},
		Server: ServerConfig{
			Port:              getEnvInt("SERVER_PORT", 8080),
			ReadTimeout:       getEnvDuration("SERVER_READ_TIMEOUT", 10*time.Second),
			ReadHeaderTimeout: getEnvDuration("SERVER_READ_HEADER_TIMEOUT", 5*time.Second),
			WriteTimeout:      getEnvDuration("SERVER_WRITE_TIMEOUT", 30*time.Second),
			IdleTimeout:       getEnvDuration("SERVER_IDLE_TIMEOUT", 120*time.Second),
			ShutdownTimeout:   getEnvDuration("SERVER_SHUTDOWN_TIMEOUT", 30*time.Second),
		},
		Database: DatabaseConfig{
			DBSchema: getEnvStr("POSTGRES_DB_SCHEMA", ""),
			URL:      getEnvStr("POSTGRES_URL", ""),
			DBDriver: getEnvStr("POSTGRES_DRIVER", ""),
		},
		Oauth: OauthConfig{
			GoogleClientId:     getEnvStr("GOOGLE_CLIENT_ID", ""),
			GoogleClientSecret: getEnvStr("GOOGLE_CLIENT_SECRET", ""),
		},
		OTel: OTelConfig{
			Endpoint:            getEnvStr("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
			SampleRatio:         getEnvFloat64("OTEL_TRACES_SAMPLER_RATIO", 0.1),
			OtelShutdownTimeout: getEnvDuration("", 5),
			// Headers map[string]string
			TLSCAFile:   getEnvStr("", ""),
			TLSCertFile: getEnvStr("", ""),
			TLSKeyFile:  getEnvStr("", ""),
		},
	}

	return cfg, nil
}
