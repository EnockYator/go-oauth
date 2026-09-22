// Package response provides consistent HTTP JSON responses for the application.
package response

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/EnockYator/go-oauth/internal/shared/apperror"
	"github.com/EnockYator/go-oauth/internal/shared/requestcontext"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// WriteJSON encodes data as JSON and writes it to the HTTP response.
//
// The response is not committed until JSON encoding succeeds. This prevents
// an encoding failure from leaving the client with a partially initialized
// HTTP response.
func writeJSON(w http.ResponseWriter, status int, data any) error {
	body, err := json.Marshal(data)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_, err = w.Write(body)
	if err != nil {
		return err
	}

	return nil
}

// WriteSuccess writes a successful JSON response.
func WriteResponse(w http.ResponseWriter, status int, data any) {
	err := writeJSON(w, status, APISuccessResponse{
		Success: true,
		Data:    data,
	})
	if err != nil {
		slog.Error(
			"failed to write success response",
			"error", err,
			"status", status,
		)
	}
}

// WriteError converts an application error into a standardized HTTP error
// response.
//
// Known application errors are translated according to their application
// error code. Unknown errors are treated as internal server errors and their
// underlying details are never exposed to the client.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	var appErr *apperror.AppError

	// Determine whether this is a known application error.
	if errors.As(err, &appErr) {
		recordAppError(r.Context(), appErr)
		writeAppError(w, r, appErr)
		return
	}

	// Unknown errors are unexpected and should be treated as bugs.
	recordUnknownError(r.Context(), err)
	writeUnknownError(w, r, err)
}

// recordAppError records a known application error in the current OpenTelemetry span.
func recordAppError(ctx context.Context, appErr *apperror.AppError) {
	span := trace.SpanFromContext(ctx)

	if !span.SpanContext().IsValid() {
		return
	}

	// Record the underlying exception if it exists, otherwise record the application error message.
	if appErr.Err != nil {
		span.RecordError(appErr.Err)
	} else {
		span.RecordError(errors.New(appErr.Message))
	}

	// Mark the span status as an error and provide a reason
	// code.Error makes the UI show the error in red, which is appropriate for application errors.
	span.SetStatus(codes.Error, appErr.Message)

	span.SetAttributes(
		attribute.String("error.code", string(appErr.Code)),
		attribute.Int("http.status_code", statusFromCode(appErr.Code)),
		attribute.String("error.message", appErr.Message),
		attribute.String("error.request_id", appErr.RequestID),
		attribute.Bool("error.is_bug", false),
	)
}

func recordUnknownError(ctx context.Context, err error) {
	span := trace.SpanFromContext(ctx)

	if !span.SpanContext().IsValid() {
		return
	}

	span.RecordError(err)

	span.SetStatus(codes.Error, "unknown error")

	span.SetAttributes(
		attribute.String("error.code", string(apperror.CodeInternalServerError)),
		attribute.Int("http.status_code", statusFromCode(apperror.CodeInternalServerError)),
		attribute.String("error.message", "internal server error"),
		attribute.Bool("error.is_bug", true),
	)
}

// writeAppError writes a known application error as an HTTP response.
func writeAppError(
	w http.ResponseWriter,
	r *http.Request,
	err *apperror.AppError,
) {
	errdata := APIErrorResponse{
		Code:      err.Code,
		Message:   err.Message,
		Details:   err.Details,
		RequestID: requestcontext.GetRequestID(r.Context()),
	}

	if writeErr := writeJSON(
		w,
		statusFromCode(err.Code),
		errdata,
	); writeErr != nil {
		slog.ErrorContext(
			r.Context(),
			"failed to write application error response",
			"error", writeErr,
			"app_error_code", err.Code,
		)
	}
}

// writeUnknownError logs the original error and writes a generic internal
// server error response to the client.
func writeUnknownError(
	w http.ResponseWriter,
	r *http.Request,
	originalErr error,
) {
	slog.ErrorContext(
		r.Context(),
		"unhandled internal server error",
		"error", originalErr,
	)

	response := APIErrorResponse{
		Code:      apperror.CodeInternalServerError,
		Message:   "internal server error",
		RequestID: requestcontext.GetRequestID(r.Context()),
	}

	if writeErr := writeJSON(
		w,
		statusFromCode(apperror.CodeInternalServerError),
		response,
	); writeErr != nil {
		slog.ErrorContext(
			r.Context(),
			"failed to write internal server error response",
			"error", writeErr,
		)
	}
}
