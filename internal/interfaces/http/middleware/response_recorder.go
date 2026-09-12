package middleware

import (
	"net/http"
)

// responseRecorder wraps a standard http.ResponseWriter to track metadata.
//
// It records:
//   - HTTP status code
//   - number of bytes written
//   - whether the response has been committed
type responseRecorder struct {
	http.ResponseWriter       // Embedded interface: inherits all ResponseWriter methods
	status              int   // Captures the HTTP status code (e.g., 200, 404)
	bytes               int64 // Tracks total bytes written to the client
	committed           bool  // Flag to prevent writing headers multiple times
}

// WriteHeader captures the status code before passing it to the real writer.
func (rw *responseRecorder) WriteHeader(status int) {
	if rw.committed {
		return // Header already sent, do nothing
	}

	rw.status = status
	rw.committed = true

	// Pass the call down to the original, underlying ResponseWriter
	rw.ResponseWriter.WriteHeader(status)
}

// Write tracks the number of bytes written and defaults the status to 200 if unset.
func (rw *responseRecorder) Write(b []byte) (int, error) {
	// If WriteHeader wasn't called explicitly, Go defaults to 200 OK
	if !rw.committed {
		rw.WriteHeader(http.StatusOK)
	}

	// Write the data to the client using the real writer
	n, err := rw.ResponseWriter.Write(b)

	// Accumulate the bytes written during the response lifecycle
	rw.bytes += int64(n)
	return n, err
}

// Unwrap allows http.ResponseController and other standard-library
// facilities to reach the underlying ResponseWriter.
func (rw *responseRecorder) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

// Status returns the HTTP status code of the response, defaulting to 200 if not set.
func (rw *responseRecorder) Status() int {
	if rw.status == 0 {
		return http.StatusOK
	}

	return rw.status
}

// BytesWritten returns the total number of bytes written during the response lifecycle.
func (rw *responseRecorder) BytesWritten() int64 {
	return rw.bytes
}

// Committed indicates whether the response headers have been sent to the client.
func (rw *responseRecorder) Committed() bool {
	return rw.committed
}
