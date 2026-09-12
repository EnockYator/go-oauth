package config

import (
	"time"
)

//Load loads all the configurations required to start the application
func Load() (*AppConfig, error) {
	cfg := &AppConfig{
		AppEnv: getEnvStr("APP_ENV", "production"),
		AppName: getEnvStr("APP_NAME", ""),
		Server: ServerConfig{
			Port: getEnvInt("SERVER_PORT", 8080),
			ReadTimeout: getEnvDuration("SERVER_READ_TIMEOUT", 5 * time.Second),
			ReadHeaderTimeout: getEnvDuration("SERVER_READ_HEADER_TIMEOUT", 5 * time.Second),
			WriteTimeout: getEnvDuration("SERVER_WRITE_TIMEOUT", 10 * time.Second),
			IdleTimeout: getEnvDuration("SERVER_IDLE_TIMEOUT", 20 * time.Second),
		},
		Database: DatabaseConfig{
			    Host: getEnvStr("POSTGRES_HOST", ""),
				Port: getEnvInt("POSTGRES_PORT", 0),
				User: getEnvStr("POSTGRES_USER", ""),
				Password: getEnvStr("POSTGRES_PASSWORD", ""),
				DBName: getEnvStr("POSTGRES_DB_NAME", ""),
				DBSchema: getEnvStr("POSTGRES_DB_SCHEMA", ""),
				SSLMode: getEnvStr("POSTGRES_SSLMODE", ""),
				URL: getEnvStr("POSTGRES_URL", ""),
				DBDriver: getEnvStr("POSTGRES_DRIVER", ""),
		},
		Oauth: OauthConfig{
			GoogleClientId: getEnvStr("GOOGLE_CLIENT_ID", ""),
    		GoogleClientSecret: getEnvStr("GOOGLE_CLIENT_SECRET", ""),
		},
	}

	return cfg, nil
}