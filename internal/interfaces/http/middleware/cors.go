package middleware

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/EnockYator/go-oauth/internal/interfaces/http/response"
	"github.com/EnockYator/go-oauth/internal/shared/apperror"
)

// CORSConfig defines the application's Cross-Origin Resource Sharing policy.
// It is used to configure the CORS middleware.
//   - AllowedOrigins: A list of allowed origins. Wildcard origins ("*") are not supported.
//   - AllowedMethods: A list of allowed HTTP methods.
//   - AllowedHeaders: A list of allowed request headers that browsers may send.
//   - AllowCredentials: Whether to allow credentials in cross-origin requests.
//     Allows browsers to include credentials such as
//     cookies or authorization credentials in cross-origin requests.
//   - MaxAge: How long browsers may cache successful preflight responses, in seconds.
type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	AllowCredentials bool
	MaxAge           int
}

// corsPolicy is the normalized, immutable CORS policy used at request time.
type corsPolicy struct {
	origins map[string]struct{}
	methods map[string]struct{}
	headers map[string]struct{}

	allowMethods     string
	allowHeaders     string
	allowCredentials bool
	maxAge           string
}

// NewCORS validates and normalizes the supplied CORS configuration.
//
// Configuration errors are returned at startup rather than being discovered
// while serving requests.
func NewCORS(cfg CORSConfig) (func(http.Handler) http.Handler, error) {
	policy, err := newCORSPolicy(cfg)
	if err != nil {
		return nil, err
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			policy.handle(w, r, next)
		})
	}, nil
}

// newCORSPolicy validates and normalizes the CORS configuration.
func newCORSPolicy(cfg CORSConfig) (*corsPolicy, error) {
	origins, err := normalizeOrigins(cfg.AllowedOrigins)
	if err != nil {
		return nil, err
	}

	methods, err := normalizeMethods(cfg.AllowedMethods)
	if err != nil {
		return nil, err
	}

	headers, err := normalizeHeaders(cfg.AllowedHeaders)
	if err != nil {
		return nil, err
	}

	if cfg.MaxAge < 0 {
		return nil, fmt.Errorf(
			"invalid CORS configuration: max age cannot be negative",
		)
	}

	policy := &corsPolicy{
		origins:          origins,
		methods:          methods.set,
		headers:          headers.set,
		allowMethods:     strings.Join(methods.values, ", "),
		allowHeaders:     strings.Join(headers.values, ", "),
		allowCredentials: cfg.AllowCredentials,
	}

	if cfg.MaxAge > 0 {
		policy.maxAge = strconv.Itoa(cfg.MaxAge)
	}

	return policy, nil
}

// handle applies the configured CORS policy to the request.
func (p *corsPolicy) handle(
	w http.ResponseWriter,
	r *http.Request,
	next http.Handler,
) {
	origin := strings.TrimSpace(r.Header.Get("Origin"))

	// Requests without Origin are not CORS requests.
	//
	// Leave them completely untouched.
	if origin == "" {
		next.ServeHTTP(w, r)
		return
	}

	// Responses may vary depending on Origin.
	//
	// This is important when responses pass through a shared cache.
	w.Header().Add("Vary", "Origin")

	// The origin must exactly match an explicitly configured origin.
	if _, allowed := p.origins[origin]; !allowed {
		if isPreflight(r) {
			p.rejectPreflight(w, r)
			return
		}

		// For a normal cross-origin request, the browser will enforce CORS
		// because no Access-Control-Allow-Origin header is returned.
		//
		// The server still processes the request normally.
		next.ServeHTTP(w, r)
		return
	}

	if isPreflight(r) {
		if !p.validatePreflight(r) {
			p.rejectPreflight(w, r)
			return
		}

		p.setResponseHeaders(w, origin)

		w.WriteHeader(http.StatusNoContent)
		return
	}

	p.setResponseHeaders(w, origin)

	next.ServeHTTP(w, r)
}

// setResponseHeaders writes CORS response headers for an allowed origin.
func (p *corsPolicy) setResponseHeaders(
	w http.ResponseWriter,
	origin string,
) {
	w.Header().Set(
		"Access-Control-Allow-Origin",
		origin,
	)

	if p.allowCredentials {
		w.Header().Set(
			"Access-Control-Allow-Credentials",
			"true",
		)
	}

	if p.allowMethods != "" {
		w.Header().Set(
			"Access-Control-Allow-Methods",
			p.allowMethods,
		)
	}

	if p.allowHeaders != "" {
		w.Header().Set(
			"Access-Control-Allow-Headers",
			p.allowHeaders,
		)
	}

	if p.maxAge != "" {
		w.Header().Set(
			"Access-Control-Max-Age",
			p.maxAge,
		)
	}
}

// validatePreflight validates the browser's requested method and headers.
func (p *corsPolicy) validatePreflight(r *http.Request) bool {
	requestedMethod := strings.ToUpper(
		strings.TrimSpace(
			r.Header.Get("Access-Control-Request-Method"),
		),
	)

	if requestedMethod == "" {
		return false
	}

	if _, allowed := p.methods[requestedMethod]; !allowed {
		return false
	}

	for _, value := range r.Header.Values(
		"Access-Control-Request-Headers",
	) {
		for _, header := range strings.Split(value, ",") {
			header = strings.TrimSpace(header)

			if header == "" {
				continue
			}

			if strings.ContainsAny(header, " \t\r\n") {
				return false
			}

			header = http.CanonicalHeaderKey(header)

			if _, allowed := p.headers[header]; !allowed {
				return false
			}
		}
	}

	return true
}

// rejectPreflight rejects an invalid CORS preflight request.
func (p *corsPolicy) rejectPreflight(
	w http.ResponseWriter,
	r *http.Request,
) {
	response.WriteError(
		w,
		r,
		apperror.New(
			r.Context(),
			apperror.CodeForbidden,
			"CORS preflight request not allowed",
			nil,
		),
	)
}

// normalizedValues contains normalized values and a lookup set.
type normalizedValues struct {
	values []string
	set    map[string]struct{}
}

// normalizeOrigins validates and normalizes configured origins.
func normalizeOrigins(
	values []string,
) (map[string]struct{}, error) {
	origins := make(map[string]struct{}, len(values))

	for _, value := range values {
		origin := strings.TrimSpace(value)

		if origin == "" {
			continue
		}

		if origin == "*" {
			return nil, fmt.Errorf(
				"invalid CORS configuration: wildcard origins are not supported",
			)
		}

		parsed, err := url.Parse(origin)
		if err != nil {
			return nil, fmt.Errorf(
				"invalid CORS origin %q: %w",
				origin,
				err,
			)
		}

		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return nil, fmt.Errorf(
				"invalid CORS origin %q: scheme must be http or https",
				origin,
			)
		}

		if parsed.Host == "" {
			return nil, fmt.Errorf(
				"invalid CORS origin %q: missing host",
				origin,
			)
		}

		if parsed.Path != "" ||
			parsed.RawQuery != "" ||
			parsed.Fragment != "" ||
			parsed.User != nil {
			return nil, fmt.Errorf(
				"invalid CORS origin %q: origin must not contain path, query, fragment, or user information",
				origin,
			)
		}

		origins[origin] = struct{}{}
	}

	return origins, nil
}

// normalizeMethods validates and normalizes configured HTTP methods.
func normalizeMethods(
	values []string,
) (normalizedValues, error) {
	methods := normalizedValues{
		values: make([]string, 0, len(values)),
		set:    make(map[string]struct{}, len(values)),
	}

	for _, value := range values {
		method := strings.ToUpper(
			strings.TrimSpace(value),
		)

		if method == "" {
			continue
		}

		if !isValidHTTPMethod(method) {
			return normalizedValues{}, fmt.Errorf(
				"invalid CORS method %q",
				method,
			)
		}

		if _, exists := methods.set[method]; exists {
			continue
		}

		methods.set[method] = struct{}{}
		methods.values = append(methods.values, method)
	}

	return methods, nil
}

// normalizeHeaders validates and normalizes configured HTTP headers.
func normalizeHeaders(
	values []string,
) (normalizedValues, error) {
	headers := normalizedValues{
		values: make([]string, 0, len(values)),
		set:    make(map[string]struct{}, len(values)),
	}

	for _, value := range values {
		header := strings.TrimSpace(value)

		if header == "" {
			continue
		}

		if strings.ContainsAny(header, " \t\r\n") {
			return normalizedValues{}, fmt.Errorf(
				"invalid CORS header %q",
				header,
			)
		}

		header = http.CanonicalHeaderKey(header)

		if _, exists := headers.set[header]; exists {
			continue
		}

		headers.set[header] = struct{}{}
		headers.values = append(headers.values, header)
	}

	return headers, nil
}

// isPreflight determines whether the request is a CORS preflight request.
func isPreflight(r *http.Request) bool {
	return r.Method == http.MethodOptions &&
		r.Header.Get("Origin") != "" &&
		r.Header.Get("Access-Control-Request-Method") != ""
}

// isValidHTTPMethod determines whether method is an explicitly supported
// standard HTTP method.
func isValidHTTPMethod(method string) bool {
	switch method {
	case http.MethodGet,
		http.MethodHead,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
		http.MethodConnect,
		http.MethodOptions,
		http.MethodTrace:
		return true
	default:
		return false
	}
}
