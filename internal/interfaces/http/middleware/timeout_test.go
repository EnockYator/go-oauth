package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		timeout time.Duration
		wantErr bool
	}{
		{
			name:    "valid timeout",
			timeout: 30 * time.Second,
			wantErr: false,
		},
		{
			name:    "minimum positive timeout",
			timeout: time.Nanosecond,
			wantErr: false,
		},
		{
			name:    "zero timeout",
			timeout: 0,
			wantErr: true,
		},
		{
			name:    "negative timeout",
			timeout: -time.Second,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			middleware, err := NewTimeout(tt.timeout)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				if middleware != nil {
					t.Fatal("expected nil middleware when configuration is invalid")
				}

				return
			}

			if err != nil {
				t.Fatalf("NewTimeout() error = %v", err)
			}

			if middleware == nil {
				t.Fatal("expected middleware, got nil")
			}
		})
	}
}

func TestTimeoutMiddleware_PropagatesDeadline(t *testing.T) {
	t.Parallel()

	const timeout = 100 * time.Millisecond

	middleware, err := NewTimeout(timeout)
	if err != nil {
		t.Fatalf("NewTimeout() error = %v", err)
	}

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deadline, ok := r.Context().Deadline()
		if !ok {
			t.Fatal("expected request context to contain a deadline")
		}

		remaining := time.Until(deadline)

		if remaining <= 0 {
			t.Fatalf("deadline has already expired: %v", remaining)
		}

		if remaining > timeout {
			t.Fatalf(
				"remaining deadline = %v, want <= %v",
				remaining,
				timeout,
			)
		}

		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(
		http.MethodGet,
		"/",
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
}

func TestTimeoutMiddleware_CancelsContext(t *testing.T) {
	t.Parallel()

	const timeout = 20 * time.Millisecond

	middleware, err := NewTimeout(timeout)
	if err != nil {
		t.Fatalf("NewTimeout() error = %v", err)
	}

	cancelled := make(chan struct{})

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			close(cancelled)

		case <-time.After(time.Second):
			t.Error("request context was not cancelled")
		}
	}))

	req := httptest.NewRequest(
		http.MethodGet,
		"/",
		nil,
	)

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	select {
	case <-cancelled:
		// Expected.
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for request context cancellation")
	}
}

func TestTimeoutMiddleware_ContextErrorIsDeadlineExceeded(t *testing.T) {
	t.Parallel()

	const timeout = 20 * time.Millisecond

	middleware, err := NewTimeout(timeout)
	if err != nil {
		t.Fatalf("NewTimeout() error = %v", err)
	}

	errCh := make(chan error, 1)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()

		errCh <- r.Context().Err()
	}))

	req := httptest.NewRequest(
		http.MethodGet,
		"/",
		nil,
	)

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	select {
	case err := <-errCh:
		if err != context.DeadlineExceeded {
			t.Fatalf(
				"context error = %v, want %v",
				err,
				context.DeadlineExceeded,
			)
		}

	case <-time.After(time.Second):
		t.Fatal("timed out waiting for context deadline")
	}
}

func TestTimeoutMiddleware_PreservesShorterUpstreamDeadline(t *testing.T) {
	t.Parallel()

	const (
		upstreamTimeout = 20 * time.Millisecond
		middlewareLimit = 500 * time.Millisecond
	)

	middleware, err := NewTimeout(middlewareLimit)
	if err != nil {
		t.Fatalf("NewTimeout() error = %v", err)
	}

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deadline, ok := r.Context().Deadline()
		if !ok {
			t.Fatal("expected request context to contain a deadline")
		}

		remaining := time.Until(deadline)

		if remaining > 100*time.Millisecond {
			t.Fatalf(
				"upstream deadline was replaced; remaining = %v",
				remaining,
			)
		}

		w.WriteHeader(http.StatusNoContent)
	}))

	ctx, cancel := context.WithTimeout(
		context.Background(),
		upstreamTimeout,
	)
	defer cancel()

	req := httptest.NewRequest(
		http.MethodGet,
		"/",
		nil,
	).WithContext(ctx)

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf(
			"status = %d, want %d",
			rec.Code,
			http.StatusNoContent,
		)
	}
}

func TestTimeoutMiddleware_ReplacesLongerUpstreamDeadline(t *testing.T) {
	t.Parallel()

	const (
		upstreamTimeout = time.Second
		middlewareLimit = 25 * time.Millisecond
	)

	middleware, err := NewTimeout(middlewareLimit)
	if err != nil {
		t.Fatalf("NewTimeout() error = %v", err)
	}

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deadline, ok := r.Context().Deadline()
		if !ok {
			t.Fatal("expected request context to contain a deadline")
		}

		remaining := time.Until(deadline)

		if remaining <= 0 {
			t.Fatal("deadline expired too early")
		}

		if remaining > 100*time.Millisecond {
			t.Fatalf(
				"middleware timeout was not applied; remaining = %v",
				remaining,
			)
		}

		w.WriteHeader(http.StatusNoContent)
	}))

	ctx, cancel := context.WithTimeout(
		context.Background(),
		upstreamTimeout,
	)
	defer cancel()

	req := httptest.NewRequest(
		http.MethodGet,
		"/",
		nil,
	).WithContext(ctx)

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf(
			"status = %d, want %d",
			rec.Code,
			http.StatusNoContent,
		)
	}
}

func TestTimeoutMiddleware_PreservesAlreadyExpiredUpstreamContext(t *testing.T) {
	t.Parallel()

	middleware, err := NewTimeout(time.Second)
	if err != nil {
		t.Fatalf("NewTimeout() error = %v", err)
	}

	handlerCalled := make(chan struct{})

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(handlerCalled)

		if err := r.Context().Err(); err != context.DeadlineExceeded {
			t.Fatalf(
				"context error = %v, want %v",
				err,
				context.DeadlineExceeded,
			)
		}

		w.WriteHeader(http.StatusNoContent)
	}))

	ctx, cancel := context.WithDeadline(
		context.Background(),
		time.Now().Add(-time.Second),
	)
	defer cancel()

	req := httptest.NewRequest(
		http.MethodGet,
		"/",
		nil,
	).WithContext(ctx)

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	select {
	case <-handlerCalled:
		// Expected.
	case <-time.After(time.Second):
		t.Fatal("downstream handler was not called")
	}

	if rec.Code != http.StatusNoContent {
		t.Fatalf(
			"status = %d, want %d",
			rec.Code,
			http.StatusNoContent,
		)
	}
}

func TestTimeoutMiddleware_PropagatesParentCancellation(t *testing.T) {
	t.Parallel()

	middleware, err := NewTimeout(time.Second)
	if err != nil {
		t.Fatalf("NewTimeout() error = %v", err)
	}

	cancelled := make(chan struct{})

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		go func() {
			<-r.Context().Done()
			close(cancelled)
		}()

		w.WriteHeader(http.StatusNoContent)
	}))

	ctx, cancel := context.WithCancel(context.Background())

	req := httptest.NewRequest(
		http.MethodGet,
		"/",
		nil,
	).WithContext(ctx)

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	cancel()

	select {
	case <-cancelled:
		// Expected.
	case <-time.After(time.Second):
		t.Fatal("parent cancellation was not propagated")
	}
}

func TestTimeoutMiddleware_PreservesContextValues(t *testing.T) {
	t.Parallel()

	type contextKey string

	const key contextKey = "test-key"

	middleware, err := NewTimeout(time.Second)
	if err != nil {
		t.Fatalf("NewTimeout() error = %v", err)
	}

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := r.Context().Value(key)

		if got != "test-value" {
			t.Fatalf(
				"context value = %v, want %q",
				got,
				"test-value",
			)
		}

		w.WriteHeader(http.StatusNoContent)
	}))

	ctx := context.WithValue(
		context.Background(),
		key,
		"test-value",
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/",
		nil,
	).WithContext(ctx)

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
}

func TestTimeoutMiddleware_NormalRequest(t *testing.T) {
	t.Parallel()

	middleware, err := NewTimeout(time.Second)
	if err != nil {
		t.Fatalf("NewTimeout() error = %v", err)
	}

	const body = "hello"

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)

		_, err := w.Write([]byte(body))
		if err != nil {
			t.Fatalf("unexpected write error: %v", err)
		}
	}))

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
}
