package config

import (
	"time"
)

type App struct {
	AppEnv      string `validate:"required"`
	AppName string `validate:"required"`
}

type ServerConfig struct {
	Port         int `validate:"gte=1,lte=65535"`
	ReadTimeout  time.Duration `validate:"gt=0"`
	ReadHeaderTimeout  time.Duration `validate:"gt=0"`
	WriteTimeout time.Duration `validate:"gt=0"`
	IdleTimeout  time.Duration `validate:"gt=0"`
	ShutdownTimeout  time.Duration `validate:"gt=0"`
}

type DatabaseConfig struct {
	DBSchema string `validate:"required"`
	URL      string `validate:"required"`
	DBDriver string `validate:"required"`
}

type OauthConfig struct {
	GoogleClientId string `validate:"required"`
	GoogleClientSecret string `validate:"required"`
}

type OTelConfig struct {
	Endpoint string
	SampleRatio float64

	OtelShutdownTimeout time.Duration
	Headers map[string]string
	TLSCAFile string
	TLSCertFile string
	TLSKeyFile string
}

type Config struct {
	App App
	Server   ServerConfig 
	Database DatabaseConfig
	Oauth OauthConfig
	OTel OTelConfig
}