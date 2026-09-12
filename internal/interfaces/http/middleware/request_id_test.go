package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/EnockYator/go-oauth/internal/shared/requestcontext"
	"github.com/google/uuid"
)

func TestRequestIDMiddleware(t *testing.T) {
	t.Parallel()

	t.Run("generates request ID and propagates it", func(t *testing.T) {
		t.Parallel()

		handler := RequestIDMiddleware(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requestID := requestcontext.GetRequestID(r.Context())

				if requestID == "" {
					t.Fatal("expected request ID in context")
				}

				if _, err := uuid.Parse(requestID); err != nil {
					t.Fatalf(
						"expected valid UUID request ID, got %q: %v",
						requestID,
						err,
					)
				}

				if got := w.Header().Get(requestIDHeader); got != requestID {
					t.Fatalf(
						"response request ID = %q, want %q",
						got,
						requestID,
					)
				}

				w.WriteHeader(http.StatusNoContent)
			}),
		)

		req := httptest.NewRequest(
			http.MethodGet,
			"/health",
			nil,
		)

		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf(
				"status = %d, want %d",
				rec.Code,
				http.StatusNoContent,
			)
		}

		requestID := rec.Header().Get(requestIDHeader)

		if requestID == "" {
			t.Fatal("expected X-Request-ID response header")
		}

		if _, err := uuid.Parse(requestID); err != nil {
			t.Fatalf(
				"expected valid UUID request ID, got %q: %v",
				requestID,
				err,
			)
		}
	})

	t.Run("ignores client supplied request ID", func(t *testing.T) {
		t.Parallel()

		clientRequestID := "attacker-controlled-request-id"

		handler := RequestIDMiddleware(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requestID := requestcontext.GetRequestID(r.Context())

				if requestID == clientRequestID {
					t.Fatal(
						"middleware must not trust client supplied request ID",
					)
				}

				w.WriteHeader(http.StatusNoContent)
			}),
		)

		req := httptest.NewRequest(
			http.MethodGet,
			"/health",
			nil,
		)

		req.Header.Set(
			requestIDHeader,
			clientRequestID,
		)

		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		got := rec.Header().Get(requestIDHeader)

		if got == "" {
			t.Fatal("expected generated request ID")
		}

		if got == clientRequestID {
			t.Fatal(
				"middleware must not echo client supplied request ID",
			)
		}

		if _, err := uuid.Parse(got); err != nil {
			t.Fatalf(
				"expected generated UUID, got %q: %v",
				got,
				err,
			)
		}
	})

	t.Run("generates a different ID for each request", func(t *testing.T) {
		t.Parallel()

		handler := RequestIDMiddleware(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}),
		)

		req1 := httptest.NewRequest(
			http.MethodGet,
			"/health",
			nil,
		)

		rec1 := httptest.NewRecorder()

		handler.ServeHTTP(rec1, req1)

		req2 := httptest.NewRequest(
			http.MethodGet,
			"/health",
			nil,
		)

		rec2 := httptest.NewRecorder()

		handler.ServeHTTP(rec2, req2)

		id1 := rec1.Header().Get(requestIDHeader)
		id2 := rec2.Header().Get(requestIDHeader)

		if id1 == "" || id2 == "" {
			t.Fatal("expected both requests to have request IDs")
		}

		if id1 == id2 {
			t.Fatalf(
				"expected unique request IDs, both were %q",
				id1,
			)
		}
	})

	t.Run("preserves downstream response", func(t *testing.T) {
		t.Parallel()

		const body = "hello"

		handler := RequestIDMiddleware(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusCreated)

				_, err := w.Write([]byte(body))
				if err != nil {
					t.Fatalf("unexpected write error: %v", err)
				}
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

		if rec.Body.String() != body {
			t.Fatalf(
				"body = %q, want %q",
				rec.Body.String(),
				body,
			)
		}

		if rec.Header().Get(requestIDHeader) == "" {
			t.Fatal("expected X-Request-ID header")
		}
	})
}

func TestRequestIDMiddleware_ContextPropagation(t *testing.T) {
	t.Parallel()

	var (
		gotRequestID string
		gotContext   context.Context
	)

	handler := RequestIDMiddleware(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotContext = r.Context()
			gotRequestID = requestcontext.GetRequestID(
				r.Context(),
			)

			w.WriteHeader(http.StatusNoContent)
		}),
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/test",
		nil,
	)

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if gotContext == nil {
		t.Fatal("expected downstream request context")
	}

	if gotRequestID == "" {
		t.Fatal("expected request ID in downstream context")
	}

	responseRequestID := rec.Header().Get(requestIDHeader)

	if gotRequestID != responseRequestID {
		t.Fatalf(
			"context request ID = %q, response request ID = %q",
			gotRequestID,
			responseRequestID,
		)
	}

	// Verify the original request's context was not mutated.
	if requestcontext.GetRequestID(req.Context()) != "" {
		t.Fatal(
			"original request context must not contain request ID",
		)
	}
}
