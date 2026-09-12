package apperror

// ErrorDetail represents a specific detail associated with an application
// error, typically a validation failure.
//
// Example:
//
//	{
//	 "field": "email",
//	 "message": "must be a valid email address"
//	}
type ErrorDetail struct {
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

// WithDetails adds a validation or field-level error detail.
//
// This method is useful for accumulating multiple validation errors
// into a single AppError instance, allowing clients to see all issues
// in one response.
//
// Example:
//
//	if req.Email == "" {
//		return apperror.New(
//			ctx,
//			apperror.CodeValidationRequiredField,
//			"email is required",
//			nil,
//		).WithDetails(
//			"email",
//			"email is required",
//		)
//	}
func (e *AppError) WithDetails(
	field string,
	message string,
	code string,
) *AppError {
	e.Details = append(e.Details, ErrorDetail{
		Field:   field,
		Message: message,
	})

	return e
}
