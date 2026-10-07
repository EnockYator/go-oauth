package cookie

import (
	"net/http"
	"time"
)

const (
	secureSessionName = "__Host-session"
	devSessionName    = "session"
)

type SessionConfig struct {
	Secure   bool
	TTL      time.Duration
	SameSite http.SameSite
}

func (c SessionConfig) Name() string {
	if c.Secure {
		return secureSessionName
	}
	return devSessionName
}
func (c SessionConfig) sameSite() http.SameSite {
	if c.SameSite == 0 {
		return http.SameSiteLaxMode
	}
	return c.SameSite
}
func SetSession(w http.ResponseWriter, c SessionConfig, token string) {
	maxAge := int(c.TTL.Seconds())
	if maxAge < 1 {
		maxAge = 1
	}
	http.SetCookie(w, &http.Cookie{
		Name:     c.Name(),
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   c.Secure,
		SameSite: c.sameSite(),
		MaxAge:   maxAge,
		Expires:  time.Now().Add(c.TTL),
	})
}
func ClearSession(w http.ResponseWriter, c SessionConfig) {
	http.SetCookie(w, &http.Cookie{Name: c.Name(), Value: "", Path: "/", HttpOnly: true, Secure: c.Secure, SameSite: c.sameSite(), MaxAge: -1, Expires: time.Unix(0, 0)})
}
func ReadSession(r *http.Request, c SessionConfig) (string, bool) {
	v, err := r.Cookie(c.Name())
	if err != nil || v.Value == "" {
		return "", false
	}
	return v.Value, true
}

const (
	secureStateName = "__Host-oauth_state"
	devStateName    = "oauth_state"
)

type StateConfig struct {
	Secure bool
	TTL    time.Duration
}

func (c StateConfig) Name() string {
	if c.Secure {
		return secureStateName
	}
	return devStateName
}
func SetState(w http.ResponseWriter, c StateConfig, value string) {
	http.SetCookie(w, &http.Cookie{Name: c.Name(), Value: value, Path: "/", HttpOnly: true, Secure: c.Secure, SameSite: http.SameSiteLaxMode, MaxAge: int(c.TTL.Seconds()), Expires: time.Now().Add(c.TTL)})
}
func ClearState(w http.ResponseWriter, c StateConfig) {
	http.SetCookie(w, &http.Cookie{Name: c.Name(), Value: "", Path: "/", HttpOnly: true, Secure: c.Secure, SameSite: http.SameSiteLaxMode, MaxAge: -1, Expires: time.Unix(0, 0)})
}
func ReadState(r *http.Request, c StateConfig) (string, bool) {
	v, err := r.Cookie(c.Name())
	if err != nil || v.Value == "" {
		return "", false
	}
	return v.Value, true
}
