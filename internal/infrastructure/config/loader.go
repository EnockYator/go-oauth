package config

import (
	"errors"
	"fmt"
	"time"
)

// Load reads, parses, and validates the complete application configuration.
//
// Load never terminates the process. Configuration errors are returned to the
// caller so that the application composition root decides how startup fails.
func Load() (*Config, error) {
	cfg, err := load()
	if err != nil {
		return nil, fmt.Errorf("parse configuration: %w", err)
	}

	if err := ValidateConfig(*cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func load() (*Config, error) {
	var errs []error

	appEnv, err := getEnvStr("APP_ENV", "production")
	if err != nil {
		errs = append(errs, err)
	}

	appName, err := getEnvStr("APP_NAME", "")
	if err != nil {
		errs = append(errs, err)
	}

	appVersion, err := getEnvStr("APP_VERSION", "")
	if err != nil {
		errs = append(errs, err)
	}

	server, serverErrs := loadServer()
	errs = append(errs, serverErrs...)

	database, databaseErrs := loadDatabase()
	errs = append(errs, databaseErrs...)

	oauth, oauthErrs := loadOAuth(appEnv)
	errs = append(errs, oauthErrs...)

	otelConfig, otelErrs := loadOTel()
	errs = append(errs, otelErrs...)

	if err := errors.Join(errs...); err != nil {
		return nil, err
	}

	return &Config{
		App: App{
			AppEnv:     appEnv,
			AppName:    appName,
			AppVersion: appVersion,
		},
		Server:   server,
		Database: database,
		Oauth:    oauth,
		OTel:     otelConfig,
	}, nil
}

func loadServer() (ServerConfig, []error) {
	var errs []error

	port, err := getEnvInt("SERVER_PORT", 8080)
	if err != nil {
		errs = append(errs, err)
	}

	readTimeout, err := getEnvDuration(
		"SERVER_READ_TIMEOUT",
		10*time.Second,
	)
	if err != nil {
		errs = append(errs, err)
	}

	readHeaderTimeout, err := getEnvDuration(
		"SERVER_READ_HEADER_TIMEOUT",
		5*time.Second,
	)
	if err != nil {
		errs = append(errs, err)
	}

	writeTimeout, err := getEnvDuration(
		"SERVER_WRITE_TIMEOUT",
		30*time.Second,
	)
	if err != nil {
		errs = append(errs, err)
	}

	idleTimeout, err := getEnvDuration(
		"SERVER_IDLE_TIMEOUT",
		120*time.Second,
	)
	if err != nil {
		errs = append(errs, err)
	}

	shutdownTimeout, err := getEnvDuration(
		"SERVER_SHUTDOWN_TIMEOUT",
		30*time.Second,
	)
	if err != nil {
		errs = append(errs, err)
	}

	return ServerConfig{
		Port:              port,
		ReadTimeout:       readTimeout,
		ReadHeaderTimeout: readHeaderTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		ShutdownTimeout:   shutdownTimeout,
	}, errs
}

func loadDatabase() (DatabaseConfig, []error) {
	var errs []error

	schema, err := getEnvStr("POSTGRES_DB_SCHEMA", "")
	if err != nil {
		errs = append(errs, err)
	}

	url, err := getEnvStr("POSTGRES_URL", "")
	if err != nil {
		errs = append(errs, err)
	}

	driver, err := getEnvStr("POSTGRES_DRIVER", "")
	if err != nil {
		errs = append(errs, err)
	}

	return DatabaseConfig{
		DBSchema: schema,
		URL:      url,
		DBDriver: driver,
	}, errs
}

func loadOAuth(appEnv string) (OAuthConfig, []error) {
	var errs []error

	google, googleErrs := loadGoogle()
	errs = append(errs, googleErrs...)

	github, githubErrs := loadGitHub()
	errs = append(errs, githubErrs...)

	stateSecret, err := getEnvBase64("OAUTH_STATE_SECRET")
	if err != nil {
		errs = append(errs, err)
	}

	stateTTL, err := getEnvDuration(
		"OAUTH_STATE_TTL",
		10*time.Minute,
	)
	if err != nil {
		errs = append(errs, err)
	}

	sessionTTL, err := getEnvDuration(
		"OAUTH_SESSION_TTL",
		720*time.Hour,
	)
	if err != nil {
		errs = append(errs, err)
	}

	sessionCookieSecure, err := getEnvBool(
		"OAUTH_SESSION_SECURE",
		appEnv != "development",
	)
	if err != nil {
		errs = append(errs, err)
	}

	return OAuthConfig{
		Google:              google,
		GitHub:              github,
		StateSecret:         stateSecret,
		StateTTL:            stateTTL,
		SessionTTL:          sessionTTL,
		SessionCookieSecure: sessionCookieSecure,
	}, errs
}

func loadGoogle() (*OAuthProviderConfig, []error) {
	return loadOAuthProvider(
		"GOOGLE",
		[]string{"openid", "email", "profile"},
	)
}

func loadGitHub() (*OAuthProviderConfig, []error) {
	return loadOAuthProvider(
		"GITHUB",
		[]string{"read:user", "user:email"},
	)
}

func loadOAuthProvider(
	provider string,
	defaultScopes []string,
) (*OAuthProviderConfig, []error) {
	var errs []error

	prefix := "OAUTH_" + provider

	clientID, err := getEnvStr(prefix+"_CLIENT_ID", "")
	if err != nil {
		errs = append(errs, err)
	}

	// An OAuth provider is disabled when no client ID is configured.
	if clientID == "" {
		return nil, errs
	}

	clientSecret, err := getEnvStr(prefix+"_CLIENT_SECRET", "")
	if err != nil {
		errs = append(errs, err)
	}

	redirectURL, err := getEnvStr(prefix+"_REDIRECT_URL", "")
	if err != nil {
		errs = append(errs, err)
	}

	scopes, err := getEnvCSV(
		prefix+"_SCOPES",
		defaultScopes,
	)
	if err != nil {
		errs = append(errs, err)
	}

	return &OAuthProviderConfig{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       scopes,
	}, errs
}

func loadOTel() (OTelConfig, []error) {
	var errs []error

	endpoint, err := getEnvStr(
		"OTEL_EXPORTER_OTLP_ENDPOINT",
		"",
	)
	if err != nil {
		errs = append(errs, err)
	}

	sampleRatio, err := getEnvFloat64(
		"OTEL_TRACES_SAMPLER_RATIO",
		0.1,
	)
	if err != nil {
		errs = append(errs, err)
	}

	shutdownTimeout, err := getEnvDuration(
		"OTEL_SHUTDOWN_TIMEOUT",
		5*time.Second,
	)
	if err != nil {
		errs = append(errs, err)
	}

	exportTimeout, err := getEnvDuration(
		"OTEL_EXPORT_TIMEOUT",
		10*time.Second,
	)
	if err != nil {
		errs = append(errs, err)
	}

	headers, err := getEnvMap(
		"OTEL_EXPORTER_OTLP_HEADERS",
	)
	if err != nil {
		errs = append(errs, err)
	}

	caFile, err := getEnvStr(
		"OTEL_EXPORTER_OTLP_CA_FILE",
		"",
	)
	if err != nil {
		errs = append(errs, err)
	}

	certFile, err := getEnvStr(
		"OTEL_EXPORTER_OTLP_CERT_FILE",
		"",
	)
	if err != nil {
		errs = append(errs, err)
	}

	keyFile, err := getEnvStr(
		"OTEL_EXPORTER_OTLP_KEY_FILE",
		"",
	)
	if err != nil {
		errs = append(errs, err)
	}

	serverName, err := getEnvStr(
		"OTEL_EXPORTER_OTLP_TLS_SERVER_NAME",
		"",
	)
	if err != nil {
		errs = append(errs, err)
	}

	return OTelConfig{
		Endpoint:            endpoint,
		SampleRatio:         sampleRatio,
		OtelShutdownTimeout: shutdownTimeout,
		ExportTimeout:       exportTimeout,
		Headers:             headers,
		TLSCAFile:           caFile,
		TLSCertFile:         certFile,
		TLSKeyFile:          keyFile,
		TLSServerName:       serverName,
	}, errs
}
