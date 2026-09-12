package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestRecoveryMiddleware_RecoversPanic(t *testing.T) {
	t.Parallel()

	handler := RecoveryMiddleware()(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("test panic")
		}),
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/test",
		nil,
	)

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf(
			"status = %d, want %d",
			rec.Code,
			http.StatusInternalServerError,
		)
	}

	if strings.Contains(rec.Body.String(), "test panic") {
		t.Fatal("panic details leaked into response")
	}
}

func TestRecoveryMiddleware_PreservesCommittedResponse(t *testing.T) {
	t.Parallel()

	handler := RecoveryMiddleware()(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte("created"))

			panic("panic after response")
		}),
	)

	req := httptest.NewRequest(
		http.MethodPost,
		"/resource",
		nil,
	)

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf(
			"status = %d, want %d",
			rec.Code,
			http.StatusCreated,
		)
	}

	if got := rec.Body.String(); got != "created" {
		t.Fatalf(
			"body = %q, want %q",
			got,
			"created",
		)
	}
}

func TestRecoveryMiddleware_LogsPanic(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer

	handler := RecoveryMiddleware()(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("expected panic")
		}),
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/test",
		nil,
	)

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	output := logs.String()

	if !strings.Contains(
		output,
		"panic recovered from HTTP request",
	) {
		t.Fatal("expected recovery log message")
	}

	if !strings.Contains(output, "expected panic") {
		t.Fatal("expected panic value in log")
	}

	if !strings.Contains(output, "stack_trace") {
		t.Fatal("expected stack trace in log")
	}
}

func TestRecoveryMiddleware_RecordsPanicInOpenTelemetry(t *testing.T) {
	t.Parallel()

	spanRecorder := tracetest.NewSpanRecorder()

	tp := trace.NewTracerProvider(
		trace.WithSpanProcessor(spanRecorder),
	)

	tracer := tp.Tracer("recovery-test")

	handler := RecoveryMiddleware()(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("otel panic")
		}),
	)

	ctx, span := tracer.Start(t.Context(), "request")

	req := httptest.NewRequest(
		http.MethodGet,
		"/test",
		nil,
	).WithContext(ctx)

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	span.End()

	spans := spanRecorder.Ended()

	if len(spans) != 1 {
		t.Fatalf(
			"spans = %d, want 1",
			len(spans),
		)
	}

	recorded := spans[0]

	if recorded.Status().Code != codes.Error {
		t.Fatalf(
			"span status = %v, want %v",
			recorded.Status().Code,
			codes.Error,
		)
	}

	if len(recorded.Events()) == 0 {
		t.Fatal("expected panic exception event")
	}
}
