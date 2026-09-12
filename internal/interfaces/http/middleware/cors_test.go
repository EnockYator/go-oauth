package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewCORS_Configuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  CORSConfig
		wantErr bool
	}{
		{
			name: "valid configuration",
			config: CORSConfig{
				AllowedOrigins: []string{
					"https://example.com",
					"http://localhost:3000",
				},
				AllowedMethods: []string{
					http.MethodGet,
					http.MethodPost,
				},
				AllowedHeaders: []string{
					"Authorization",
					"Content-Type",
				},
				AllowCredentials: true,
				MaxAge:           3600,
			},
			wantErr: false,
		},
		{
			name: "wildcard origin is rejected",
			config: CORSConfig{
				AllowedOrigins: []string{"*"},
			},
			wantErr: true,
		},
		{
			name: "negative max age is rejected",
			config: CORSConfig{
				AllowedOrigins: []string{"https://example.com"},
				MaxAge:         -1,
			},
			wantErr: true,
		},
		{
			name: "origin with path is rejected",
			config: CORSConfig{
				AllowedOrigins: []string{
					"https://example.com/photos",
				},
			},
			wantErr: true,
		},
		{
			name: "origin with query is rejected",
			config: CORSConfig{
				AllowedOrigins: []string{
					"https://example.com?foo=bar",
				},
			},
			wantErr: true,
		},
		{
			name: "origin with fragment is rejected",
			config: CORSConfig{
				AllowedOrigins: []string{
					"https://example.com#dashboard",
				},
			},
			wantErr: true,
		},
		{
			name: "origin with user information is rejected",
			config: CORSConfig{
				AllowedOrigins: []string{
					"https://user@example.com",
				},
			},
			wantErr: true,
		},
		{
			name: "origin without host is rejected",
			config: CORSConfig{
				AllowedOrigins: []string{
					"https://",
				},
			},
			wantErr: true,
		},
		{
			name: "unsupported origin scheme is rejected",
			config: CORSConfig{
				AllowedOrigins: []string{
					"ftp://example.com",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid HTTP method is rejected",
			config: CORSConfig{
				AllowedOrigins: []string{
					"https://example.com",
				},
				AllowedMethods: []string{
					"PURGE",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid header is rejected",
			config: CORSConfig{
				AllowedOrigins: []string{
					"https://example.com",
				},
				AllowedHeaders: []string{
					"X-Invalid Header",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewCORS(tt.config)

			if (err != nil) != tt.wantErr {
				t.Fatalf(
					"NewCORS() error = %v, wantErr = %v",
					err,
					tt.wantErr,
				)
			}
		})
	}
}

func TestCORS(t *testing.T) {
	t.Parallel()

	cfg := CORSConfig{
		AllowedOrigins: []string{
			"https://example.com",
			"http://localhost:3000",
		},
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodOptions,
		},
		AllowedHeaders: []string{
			"Authorization",
			"Content-Type",
			"X-Custom-Header",
		},
		AllowCredentials: true,
		MaxAge:           3600,
	}

	middleware, err := NewCORS(cfg)
	if err != nil {
		t.Fatalf("NewCORS() unexpected error: %v", err)
	}

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("request without Origin passes through", func(t *testing.T) {
		req := httptest.NewRequest(
			http.MethodGet,
			"/photos",
			nil,
		)

		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Fatalf(
				"expected no Access-Control-Allow-Origin header, got %q",
				got,
			)
		}

		if got := rec.Header().Get("Vary"); got != "" {
			t.Fatalf(
				"expected no Vary header, got %q",
				got,
			)
		}
	})

	t.Run("allowed origin receives CORS headers", func(t *testing.T) {
		req := httptest.NewRequest(
			http.MethodGet,
			"/photos",
			nil,
		)
		req.Header.Set(
			"Origin",
			"https://example.com",
		)

		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		assertHeader(
			t,
			rec,
			"Access-Control-Allow-Origin",
			"https://example.com",
		)

		assertHeader(
			t,
			rec,
			"Access-Control-Allow-Credentials",
			"true",
		)

		assertHeader(t, rec, "Vary", "Origin")
	})

	t.Run("disallowed origin passes through without CORS permission", func(t *testing.T) {
		req := httptest.NewRequest(
			http.MethodGet,
			"/photos",
			nil,
		)
		req.Header.Set(
			"Origin",
			"https://evil.example",
		)

		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Fatalf(
				"expected no Access-Control-Allow-Origin header, got %q",
				got,
			)
		}

		assertHeader(t, rec, "Vary", "Origin")
	})

	t.Run("valid preflight returns 204 with CORS policy", func(t *testing.T) {
		req := httptest.NewRequest(
			http.MethodOptions,
			"/photos",
			nil,
		)

		req.Header.Set(
			"Origin",
			"https://example.com",
		)
		req.Header.Set(
			"Access-Control-Request-Method",
			http.MethodPost,
		)
		req.Header.Set(
			"Access-Control-Request-Headers",
			"Authorization, Content-Type, X-Custom-Header",
		)

		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf(
				"expected 204, got %d",
				rec.Code,
			)
		}

		assertHeader(
			t,
			rec,
			"Access-Control-Allow-Origin",
			"https://example.com",
		)

		assertHeader(
			t,
			rec,
			"Access-Control-Allow-Credentials",
			"true",
		)

		assertHeader(
			t,
			rec,
			"Access-Control-Allow-Methods",
			"GET, POST, OPTIONS",
		)

		assertHeader(
			t,
			rec,
			"Access-Control-Allow-Headers",
			"Authorization, Content-Type, X-Custom-Header",
		)

		assertHeader(
			t,
			rec,
			"Access-Control-Max-Age",
			"3600",
		)

		assertHeader(t, rec, "Vary", "Origin")
	})

	t.Run("preflight with disallowed method is rejected", func(t *testing.T) {
		req := newPreflightRequest(
			"https://example.com",
			http.MethodDelete,
			"",
		)

		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code == http.StatusNoContent {
			t.Fatal("expected disallowed method preflight to be rejected")
		}

		if rec.Code == http.StatusOK {
			t.Fatal("rejected preflight must not reach downstream handler")
		}

		assertHeader(t, rec, "Vary", "Origin")
	})

	t.Run("preflight with disallowed header is rejected", func(t *testing.T) {
		req := newPreflightRequest(
			"https://example.com",
			http.MethodPost,
			"Authorization, X-Malicious-Header",
		)

		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code == http.StatusNoContent {
			t.Fatal("expected disallowed header preflight to be rejected")
		}

		if rec.Code == http.StatusOK {
			t.Fatal("rejected preflight must not reach downstream handler")
		}

		assertHeader(t, rec, "Vary", "Origin")
	})

	t.Run("preflight with disallowed origin is rejected", func(t *testing.T) {
		req := newPreflightRequest(
			"https://evil.example",
			http.MethodPost,
			"Content-Type",
		)

		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code == http.StatusNoContent {
			t.Fatal("expected disallowed origin preflight to be rejected")
		}

		if rec.Code == http.StatusOK {
			t.Fatal("rejected preflight must not reach downstream handler")
		}

		assertHeader(t, rec, "Vary", "Origin")

		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Fatalf(
				"expected no Access-Control-Allow-Origin header, got %q",
				got,
			)
		}
	})

	t.Run("OPTIONS without preflight headers passes through", func(t *testing.T) {
		req := httptest.NewRequest(
			http.MethodOptions,
			"/photos",
			nil,
		)
		req.Header.Set(
			"Origin",
			"https://example.com",
		)

		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf(
				"expected 200 from downstream handler, got %d",
				rec.Code,
			)
		}

		assertHeader(
			t,
			rec,
			"Access-Control-Allow-Origin",
			"https://example.com",
		)

		assertHeader(t, rec, "Vary", "Origin")
	})
}

func newPreflightRequest(
	origin string,
	method string,
	headers string,
) *http.Request {
	req := httptest.NewRequest(
		http.MethodOptions,
		"/photos",
		nil,
	)

	req.Header.Set("Origin", origin)
	req.Header.Set(
		"Access-Control-Request-Method",
		method,
	)

	if headers != "" {
		req.Header.Set(
			"Access-Control-Request-Headers",
			headers,
		)
	}

	return req
}

func assertHeader(
	t *testing.T,
	rec *httptest.ResponseRecorder,
	name string,
	want string,
) {
	t.Helper()

	if got := rec.Header().Get(name); got != want {
		t.Fatalf(
			"header %q = %q, want %q",
			name,
			got,
			want,
		)
	}
}
