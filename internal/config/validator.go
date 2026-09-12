package config

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/go-playground/validator"
)

var validate = validator.New()

// ValidateConfig checks if the loaded configuration is valid
func ValidateConfig(cfg AppConfig) error {
	if err := validate.Struct(cfg); err != nil {
		var validationErrors validator.ValidationErrors
		
		if !errors.As(err, &validationErrors) {
			slog.Error(
				"app config validation error",
			slog.Any("env_validation", err))
			return fmt.Errorf("validate config: %w", err)
		}

	return formatValidationErrors(validationErrors)
	}
	return nil
}

func formatValidationErrors(errs validator.ValidationErrors) error {
	messages := make([]string, 0, len(errs))

	for _, err := range errs {
		field := err.Namespace()
		
		switch err.Tag() {
		case "required":
			messages = append(
				messages,
				fmt.Sprintf("%s is required", field),
			)
		case "gt":
			messages = append(
				messages,
				fmt.Sprintf("%s must be greater than zero", field),
			)
		default:
			messages = append(
				messages,
				fmt.Sprintf(
					"%s failed validation: %s",
					field,
					err.Tag(),
				),
			)
		}
	}
	return fmt.Errorf(
		"configuration validation failed: %s",
		strings.Join(messages, "; "),
	)
}