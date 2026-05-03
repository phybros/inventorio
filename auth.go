package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type currentUserContextKey struct{}
type authModeContextKey struct{}

type CurrentUser struct {
	ID          string
	Email       string
	DisplayName string
	AvatarURL   string
	Provider    string
}

type AuthenticatedIdentity struct {
	Provider        string
	ProviderSubject string
	ProviderLogin   string
	Email           string
	DisplayName     string
	AvatarURL       string
}

func currentUser(r *http.Request) *CurrentUser {
	user, _ := r.Context().Value(currentUserContextKey{}).(*CurrentUser)
	return user
}

func requireCurrentUser(r *http.Request) (*CurrentUser, bool) {
	user := currentUser(r)
	return user, user != nil
}

func withCurrentUser(r *http.Request, user *CurrentUser) *http.Request {
	ctx := context.WithValue(r.Context(), currentUserContextKey{}, user)
	return r.WithContext(ctx)
}

func withAuthMode(r *http.Request, mode string) *http.Request {
	ctx := context.WithValue(r.Context(), authModeContextKey{}, mode)
	return r.WithContext(ctx)
}

func (app *App) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mode := authModeDisabled
		if app.auth != nil {
			mode = app.auth.Mode
		}
		r = withAuthMode(r, mode)
		if app.auth == nil || app.auth.Mode == authModeDisabled || publicAuthRoute(r) {
			next.ServeHTTP(w, r)
			return
		}

		switch app.auth.Mode {
		case authModeOAuth:
			user, err := app.userForSession(r)
			if err != nil {
				app.rejectUnauthenticated(w, r)
				return
			}
			next.ServeHTTP(w, withCurrentUser(r, user))
		case authModeProxy:
			user, err := app.userForProxyRequest(r)
			if err != nil {
				app.rejectUnauthenticated(w, r)
				return
			}
			next.ServeHTTP(w, withCurrentUser(r, user))
		default:
			next.ServeHTTP(w, r)
		}
	})
}

func publicAuthRoute(r *http.Request) bool {
	if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/static/") {
		return true
	}
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/login":
		return true
	case (r.Method == http.MethodGet || r.Method == http.MethodHead) && r.URL.Path == "/healthz":
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/auth/github/start":
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/auth/github/callback":
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/auth/google/start":
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/auth/google/callback":
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/logout":
		return true
	default:
		return false
	}
}

func (app *App) rejectUnauthenticated(w http.ResponseWriter, r *http.Request) {
	nextPath := sanitizeNext(r.URL.RequestURI())
	loginPath := "/login?next=" + url.QueryEscape(nextPath)
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", loginPath)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if wantsJSON(r) || strings.HasPrefix(r.URL.Path, "/api/") {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	http.Redirect(w, r, loginPath, http.StatusFound)
}

func wantsJSON(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	contentType := r.Header.Get("Content-Type")
	return strings.Contains(accept, "application/json") || strings.Contains(contentType, "application/json")
}

func (app *App) userForProxyRequest(r *http.Request) (*CurrentUser, error) {
	email, err := normalizeEmail(r.Header.Get("X-Forwarded-User"))
	if err != nil {
		return nil, err
	}
	if !app.auth.EmailAllowed(email) {
		return nil, fmt.Errorf("proxy user %q is not allowed", email)
	}
	identity := AuthenticatedIdentity{
		Provider:        "proxy",
		ProviderSubject: email,
		ProviderLogin:   email,
		Email:           email,
		DisplayName:     email,
	}
	user, err := app.upsertAuthenticatedUser(r.Context(), identity)
	if err != nil {
		log.Printf("proxy auth failed to upsert user: %v", err)
		return nil, err
	}
	return user, nil
}

func randomToken(byteLen int) (string, error) {
	buf := make([]byte, byteLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func (app *App) sessionCookieSecure(r *http.Request) bool {
	switch app.auth.CookieSecure {
	case "true":
		return true
	case "false":
		return false
	default:
		if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
			return true
		}
		return strings.HasPrefix(app.auth.PublicURL, "https://")
	}
}

func (app *App) sessionCookie(token string, expires time.Time, r *http.Request) *http.Cookie {
	return &http.Cookie{
		Name:     app.auth.SessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		MaxAge:   int(time.Until(expires).Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   app.sessionCookieSecure(r),
	}
}

func (app *App) expiredSessionCookie(r *http.Request) *http.Cookie {
	return &http.Cookie{
		Name:     app.auth.SessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   app.sessionCookieSecure(r),
	}
}
