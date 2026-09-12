package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/EnockYator/go-oauth/internal/shared/requestcontext"
)

func TestTenantMiddleware_AllowsTenant(t *testing.T) {
	t.Parallel()

	handler := TenantMiddleware(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	ctx := requestcontext.WithTenantID(t.Context(), "tenant-123")

	req := httptest.NewRequest(http.MethodGet, "/test", nil).
		WithContext(ctx)

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf(
			"status = %d, want %d",
			rec.Code,
			http.StatusOK,
		)
	}
}

func TestTenantMiddleware_RejectsMissingTenant(t *testing.T) {
	t.Parallel()

	handler := TenantMiddleware(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("next handler should not be called")
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf(
			"status = %d, want %d",
			rec.Code,
			http.StatusInternalServerError,
		)
	}
}

func TestTenantMiddleware_PreservesTenant(t *testing.T) {
	t.Parallel()

	const tenantID = "tenant-123"

	handler := TenantMiddleware(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got := requestcontext.GetTenantID(r.Context())

			if got != tenantID {
				t.Fatalf(
					"tenant ID = %q, want %q",
					got,
					tenantID,
				)
			}

			w.WriteHeader(http.StatusOK)
		}),
	)

	ctx := requestcontext.WithTenantID(t.Context(), tenantID)

	req := httptest.NewRequest(http.MethodGet, "/test", nil).
		WithContext(ctx)

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf(
			"status = %d, want %d",
			rec.Code,
			http.StatusOK,
		)
	}
}
