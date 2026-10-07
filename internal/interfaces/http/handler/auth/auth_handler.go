// Package auth exposes the OAuth login, callback, logout, and current-user
// endpoints.
package auth

import (
	"context"
	"net/http"

	"github.com/EnockYator/go-oauth/internal/interfaces/http/cookie"
	authdto "github.com/EnockYator/go-oauth/internal/interfaces/http/dto/auth"
	"github.com/EnockYator/go-oauth/internal/interfaces/http/middleware"
	"github.com/EnockYator/go-oauth/internal/interfaces/http/response"
	authapplication "github.com/EnockYator/go-oauth/internal/modules/auth/application"
	authdomain "github.com/EnockYator/go-oauth/internal/modules/auth/domain"
	"github.com/EnockYator/go-oauth/internal/shared/apperror"
)

// Handler wires the auth use cases to HTTP.
type InitiateLoginUseCase interface {
	Execute(context.Context, authapplication.InitiateLoginInput) (*authapplication.InitiateLoginOutput, error)
}

type HandleCallbackUseCase interface {
	Execute(context.Context, authapplication.HandleCallbackInput) (*authapplication.HandleCallbackOutput, error)
}

type LogoutUseCase interface {
	Execute(context.Context, string) error
}

type Handler struct {
	initiate      InitiateLoginUseCase
	callback      HandleCallbackUseCase
	logout        LogoutUseCase
	sessionCookie cookie.SessionConfig
	stateCookie   cookie.StateConfig
}

// NewHandler constructs the HTTP handler.
func NewHandler(
	initiate InitiateLoginUseCase,
	callback HandleCallbackUseCase,
	logout LogoutUseCase,
	sessionCookie cookie.SessionConfig,
	stateCookie cookie.StateConfig,
) *Handler {
	return &Handler{
		initiate:      initiate,
		callback:      callback,
		logout:        logout,
		sessionCookie: sessionCookie,
		stateCookie:   stateCookie,
	}
}

// Login handles GET /auth/login/{provider}.
//
// It sets the state cookie and redirects the browser to the provider's
// authorization endpoint.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	provider := authdomain.ProviderID(r.PathValue("provider"))

	out, err := h.initiate.Execute(r.Context(), authapplication.InitiateLoginInput{
		Provider: provider,
		ReturnTo: r.URL.Query().Get("return_to"),
	})
	if err != nil {
		response.WriteError(w, r, err)
		return
	}

	cookie.SetState(w, h.stateCookie, out.StateCookie)

	http.Redirect(w, r, out.RedirectURL, http.StatusFound)
}

// Callback handles GET /auth/callback/{provider}.
//
// It validates the state cookie, completes the code exchange, creates a
// session, and redirects the browser to the caller's return path.
func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	provider := authdomain.ProviderID(r.PathValue("provider"))
	q := r.URL.Query()

	// The user denied consent or the provider returned an error. Clear the
	// state cookie regardless.
	if errParam := q.Get("error"); errParam != "" {
		cookie.ClearState(w, h.stateCookie)
		response.WriteError(w, r, apperror.New(
			r.Context(),
			apperror.CodeUnauthorized,
			"oauth provider returned an error: "+errParam,
			nil,
		))
		return
	}

	stateCookie, ok := cookie.ReadState(r, h.stateCookie)
	if !ok {
		response.WriteError(w, r, apperror.New(
			r.Context(),
			apperror.CodeUnauthorized,
			"missing oauth state cookie",
			nil,
		))
		return
	}

	out, err := h.callback.Execute(r.Context(), authapplication.HandleCallbackInput{
		Provider:    provider,
		StateParam:  q.Get("state"),
		CodeParam:   q.Get("code"),
		StateCookie: stateCookie,
	})

	// Always clear the state cookie, success or failure: it is single-use.
	cookie.ClearState(w, h.stateCookie)

	if err != nil {
		response.WriteError(w, r, err)
		return
	}

	cookie.SetSession(w, h.sessionCookie, out.SessionToken)

	http.Redirect(w, r, out.ReturnTo, http.StatusFound)
}

// Logout handles POST /auth/logout.
//
// The endpoint is idempotent and returns 204 whether or not a valid
// session was presented.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var revokeErr error
	if raw, ok := cookie.ReadSession(r, h.sessionCookie); ok {
		revokeErr = h.logout.Execute(r.Context(), raw)
	}

	// Clear the browser cookie regardless of persistence outcome. If the
	// database operation failed, still report that failure so the client can
	// retry; clearing the cookie prevents the browser from continuing to
	// present the session automatically.
	cookie.ClearSession(w, h.sessionCookie)
	if revokeErr != nil {
		response.WriteError(w, r, revokeErr)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Me handles GET /auth/me.
//
// AuthMiddleware guarantees a user is present in context; a missing user
// is therefore a programming error, not a client error.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.GetUser(r.Context())
	if !ok {
		response.WriteError(w, r, apperror.New(
			r.Context(),
			apperror.CodeInternalServerError,
			"authenticated user missing from context",
			nil,
		))
		return
	}

	response.WriteResponse(w, http.StatusOK, authdto.NewUserResponse(u))
}
