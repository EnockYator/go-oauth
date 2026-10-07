package config

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

// ValidateConfig validates cfg and returns a descriptive error on the first
// failure. Callers are expected to treat a non-nil return as fatal.
func ValidateConfig(cfg Config) error {
	if err := validate.Struct(cfg); err != nil {
		var validationErrors validator.ValidationErrors
		if !errors.As(err, &validationErrors) {
			slog.Error(
				"config validation failed with non-field error",
				slog.Any("error", err),
			)
			return fmt.Errorf("validate config: %w", err)
		}
		return formatValidationErrors(validationErrors)
	}

	// Cross-field checks that go-playground/validator cannot express.
	if err := validateOAuthAggregate(cfg.Oauth); err != nil {
		return err
	}

	return nil
}

// validateOAuthAggregate enforces invariants spanning multiple fields.
func validateOAuthAggregate(cfg OAuthConfig) error {
	if cfg.Google == nil && cfg.GitHub == nil {
		return errors.New(
			"oauth configuration: at least one provider " +
				"(Google or GitHub) must be configured",
		)
	}

	// The __Host- cookie prefix is only honored by browsers over HTTPS.
	// Combining an insecure session cookie with a production environment
	// would silently break login for every user.
	if !cfg.SessionCookieSecure {
		slog.Warn(
			"OAUTH_SESSION_SECURE=false: session cookies will not carry " +
				"the Secure attribute; only acceptable for local development",
		)
	}

	return nil
}

// formatValidationErrors renders validator failures as a single error whose
// message lists every offending field.
func formatValidationErrors(errs validator.ValidationErrors) error {
	messages := make([]string, 0, len(errs))

	for _, err := range errs {
		field := err.Namespace()

		switch err.Tag() {
		case "required":
			messages = append(messages,
				fmt.Sprintf("%s is required", field))
		case "gt":
			messages = append(messages,
				fmt.Sprintf("%s must be greater than %s", field, err.Param()))
		case "gte":
			messages = append(messages,
				fmt.Sprintf("%s must be greater than or equal to %s", field, err.Param()))
		case "lte":
			messages = append(messages,
				fmt.Sprintf("%s must be less than or equal to %s", field, err.Param()))
		case "min":
			messages = append(messages,
				fmt.Sprintf("%s must have a minimum of %s", field, err.Param()))
		case "url":
			messages = append(messages,
				fmt.Sprintf("%s must be a valid URL", field))
		case "oneof":
			messages = append(messages,
				fmt.Sprintf("%s must be one of [%s]", field, err.Param()))
		default:
			messages = append(messages,
				fmt.Sprintf("%s failed validation: %s", field, err.Tag()))
		}
	}

	return fmt.Errorf(
		"configuration validation failed: %s",
		strings.Join(messages, "; "),
	)
}
