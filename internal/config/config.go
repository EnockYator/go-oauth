package config

import (
	"time"
)

type ServerConfig struct {
	Port         int `validate:"gte=1,lte=65535"`
	ReadTimeout  time.Duration `validate:"gt=0"`
	ReadHeaderTimeout  time.Duration `validate:"gt=0"`
	WriteTimeout time.Duration `validate:"gt=0"`
	IdleTimeout  time.Duration `validate:"gt=0"`
}

type DatabaseConfig struct {
	Host     string `validate:"required"`
	Port     int `validate:"gte=1,lte=65535"`
	User     string `validate:"required"`
	Password string `validate:"required"`
	DBName   string `validate:"required"`
	DBSchema string `validate:"required"`
	SSLMode  string `validate:"required"`
	URL      string `validate:"required"`
	DBDriver string `validate:"required"`
}

type OauthConfig struct {
	GoogleClientId string `validate:"required"`
	GoogleClientSecret string `validate:"required"`
}

type AppConfig struct {
	AppEnv      string `validate:"required"`
	AppName string `validate:"required"`
	Server   ServerConfig 
	Database DatabaseConfig
	Oauth OauthConfig
}