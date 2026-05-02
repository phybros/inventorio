package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAuthMiddlewareDisabledAllowsProtectedRoute(t *testing.T) {
	app := &App{auth: &AuthConfig{Mode: authModeDisabled}}
	called := false
	handler := app.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/components", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Fatal("next handler was not called")
	}
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
}

func TestAuthMiddlewareOAuthRedirectsBrowser(t *testing.T) {
	app := &App{auth: &AuthConfig{
		Mode:              authModeOAuth,
		SessionCookieName: "inventorio_session",
		AllowedEmails:     map[string]struct{}{"alice@example.com": {}},
	}}
	handler := app.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/components?q=led", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusFound)
	}
	if got := rr.Header().Get("Location"); got != "/login?next=%2Fcomponents%3Fq%3Dled" {
		t.Fatalf("Location = %q", got)
	}
}

func TestAuthMiddlewareOAuthRedirectsHTMX(t *testing.T) {
	app := &App{auth: &AuthConfig{Mode: authModeOAuth, SessionCookieName: "inventorio_session"}}
	handler := app.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/components", nil)
	req.Header.Set("HX-Request", "true")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
	if got := rr.Header().Get("HX-Redirect"); got != "/login?next=%2Fcomponents" {
		t.Fatalf("HX-Redirect = %q", got)
	}
}

func TestAuthMiddlewareOAuthRejectsJSON(t *testing.T) {
	app := &App{auth: &AuthConfig{Mode: authModeOAuth, SessionCookieName: "inventorio_session"}}
	handler := app.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/components/quick-create", nil)
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestAuthMiddlewareSkipsStaticAndAuthRoutes(t *testing.T) {
	app := &App{auth: &AuthConfig{Mode: authModeOAuth, SessionCookieName: "inventorio_session"}}
	calls := 0
	handler := app.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusNoContent)
	}))

	for _, path := range []string{"/static/app.css", "/login", "/auth/github/start", "/auth/google/callback"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusNoContent {
			t.Fatalf("%s status = %d, want %d", path, rr.Code, http.StatusNoContent)
		}
	}
	if calls != 4 {
		t.Fatalf("calls = %d, want 4", calls)
	}
}

func TestSessionTokenHashDoesNotStoreRawToken(t *testing.T) {
	token := "raw-session-token"
	hash := hashSessionToken(token)
	if hash == token {
		t.Fatal("hashSessionToken returned the raw token")
	}
	if len(hash) != 64 {
		t.Fatalf("hash length = %d, want 64", len(hash))
	}
}

func TestSessionCookieAttributes(t *testing.T) {
	app := &App{auth: &AuthConfig{
		SessionCookieName: "inventorio_session",
		CookieSecure:      "true",
	}}
	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	cookie := app.sessionCookie("token", time.Now().Add(time.Hour), req)

	if !cookie.HttpOnly {
		t.Fatal("session cookie is not HttpOnly")
	}
	if !cookie.Secure {
		t.Fatal("session cookie is not Secure")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("SameSite = %v, want Lax", cookie.SameSite)
	}
}

func TestRendererLoadsTemplates(t *testing.T) {
	renderer := NewRenderer()
	if _, ok := renderer.pages["auth/login"]; !ok {
		t.Fatal("auth/login template was not loaded")
	}
}

func TestRendererRendersPageDataWrapper(t *testing.T) {
	renderer := NewRenderer()
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	req = withAuthMode(req, authModeOAuth)
	rr := httptest.NewRecorder()

	renderer.RenderPage(rr, req, "auth/login", loginPageData{
		Next:             "/components",
		GitHubConfigured: true,
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestRendererHidesProtectedNavWhenLoggedOutWithAuthEnabled(t *testing.T) {
	renderer := NewRenderer()
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	req = withAuthMode(req, authModeOAuth)
	rr := httptest.NewRecorder()

	renderer.RenderPage(rr, req, "auth/login", loginPageData{
		Next:             "/components",
		GitHubConfigured: true,
	})

	body := rr.Body.String()
	for _, hidden := range []string{`href="/components" class="nav-link"`, `name="q"`, `Audit Log`} {
		if strings.Contains(body, hidden) {
			t.Fatalf("logged-out auth navbar contains %q", hidden)
		}
	}
	if !strings.Contains(body, `title="Toggle dark mode"`) {
		t.Fatal("theme switcher was not rendered")
	}
}
