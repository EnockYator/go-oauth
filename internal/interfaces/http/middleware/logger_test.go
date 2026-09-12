package middleware

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/EnockYator/go-oauth/internal/shared/requestcontext"
	"go.opentelemetry.io/otel/trace"
)

func TestLoggerMiddleware(t *testing.T) {
	t.Run("logs HTTP request details", func(t *testing.T) {
		var logs bytes.Buffer

		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte("created"))
		})

		req := httptest.NewRequest(
			http.MethodPost,
			"/api/v1/photos",
			nil,
		)

		req.RemoteAddr = "192.168.1.10:54321"
		req.Header.Set("User-Agent", "TestBrowser/1.0")

		ctx := requestcontext.WithRequestID(
			req.Context(),
			"req-123",
		)

		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()

		LoggerMiddleware()(next).ServeHTTP(rec, req)

		output := logs.String()

		expectedFields := []string{
			"http request completed",
			"request_id=req-123",
			"trace_id=\"\"",
			"method=POST",
			"path=/api/v1/photos",
			"remote_addr=192.168.1.10:54321",
			"user_agent=TestBrowser/1.0",
			"status=201",
			"bytes=7",
			"duration=",
		}

		for _, field := range expectedFields {
			if !strings.Contains(output, field) {
				t.Errorf(
					"expected log output to contain %q, got:\n%s",
					field,
					output,
				)
			}
		}
	})

	t.Run("logs valid trace ID", func(t *testing.T) {
		var logs bytes.Buffer

		traceID := trace.TraceID{
			0x01, 0x02, 0x03, 0x04,
			0x05, 0x06, 0x07, 0x08,
			0x09, 0x0a, 0x0b, 0x0c,
			0x0d, 0x0e, 0x0f, 0x10,
		}

		spanID := trace.SpanID{
			0x01, 0x02, 0x03, 0x04,
			0x05, 0x06, 0x07, 0x08,
		}

		spanContext := trace.NewSpanContext(
			trace.SpanContextConfig{
				TraceID: traceID,
				SpanID:  spanID,
			},
		)

		ctx := trace.ContextWithSpanContext(
			context.Background(),
			spanContext,
		)

		req := httptest.NewRequest(
			http.MethodGet,
			"/api/v1/photos",
			nil,
		).WithContext(ctx)

		rec := httptest.NewRecorder()

		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		LoggerMiddleware()(next).ServeHTTP(rec, req)

		output := logs.String()

		expected := "trace_id=" + traceID.String()

		if !strings.Contains(output, expected) {
			t.Errorf(
				"expected log output to contain %q, got:\n%s",
				expected,
				output,
			)
		}
	})

	t.Run("records response status and bytes", func(t *testing.T) {
		var logs bytes.Buffer

		body := "hello world"

		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(body))
		})

		req := httptest.NewRequest(
			http.MethodGet,
			"/test",
			nil,
		)

		rec := httptest.NewRecorder()

		LoggerMiddleware()(next).ServeHTTP(rec, req)

		output := logs.String()

		if !strings.Contains(output, "status=202") {
			t.Errorf(
				"expected status=202, got:\n%s",
				output,
			)
		}

		expectedBytes := "bytes=" + strconv.Itoa(len(body))

		if !strings.Contains(output, expectedBytes) {
			t.Errorf(
				"expected %s, got:\n%s",
				expectedBytes,
				output,
			)
		}
	})

	t.Run("uses default logger when logger is nil", func(t *testing.T) {
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(
			http.MethodGet,
			"/",
			nil,
		)

		rec := httptest.NewRecorder()

		LoggerMiddleware()(next).ServeHTTP(rec, req)
	})
}
