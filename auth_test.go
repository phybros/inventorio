package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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

func TestOAuthStateCookieRoundTripAndRejectsTampering(t *testing.T) {
	app := testOAuthApp()
	req := httptest.NewRequest(http.MethodGet, "https://inventory.example.com/login", nil)

	cookie, stateValue, err := app.newOAuthStateCookie("github", "/components?q=led", req)
	if err != nil {
		t.Fatalf("newOAuthStateCookie() error = %v", err)
	}
	if cookie.Name != oauthStateCookieName {
		t.Fatalf("state cookie name = %q", cookie.Name)
	}
	if cookie.Path != "/auth/github" {
		t.Fatalf("state cookie path = %q", cookie.Path)
	}
	if !cookie.HttpOnly {
		t.Fatal("state cookie is not HttpOnly")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("state cookie SameSite = %v, want Lax", cookie.SameSite)
	}

	state, err := app.decodeOAuthState(cookie.Value)
	if err != nil {
		t.Fatalf("decodeOAuthState() error = %v", err)
	}
	if state.Provider != "github" || state.State != stateValue || state.Next != "/components?q=led" {
		t.Fatalf("decoded state = %+v, stateValue = %q", state, stateValue)
	}

	if _, err := app.decodeOAuthState(cookie.Value + "x"); err == nil {
		t.Fatal("decodeOAuthState() accepted tampered state")
	}

	expired, err := app.encodeOAuthState(oauthState{
		Provider: "github",
		State:    "expired",
		Next:     "/",
		Expires:  time.Now().Add(-time.Minute).Unix(),
	})
	if err != nil {
		t.Fatalf("encodeOAuthState() error = %v", err)
	}
	if _, err := app.decodeOAuthState(expired); err == nil {
		t.Fatal("decodeOAuthState() accepted expired state")
	}
}

func TestOAuthStartSetsStateCookieAndRedirectsToProvider(t *testing.T) {
	app := testOAuthApp()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /auth/{provider}/start", app.HandleOAuthStart)

	req := httptest.NewRequest(http.MethodGet, "/auth/github/start?next=%2Fcomponents", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusFound)
	}
	stateCookie := findCookie(rr.Result().Cookies(), oauthStateCookieName)
	if stateCookie == nil {
		t.Fatal("OAuth start did not set state cookie")
	}
	if !stateCookie.HttpOnly {
		t.Fatal("state cookie is not HttpOnly")
	}

	location := rr.Header().Get("Location")
	u, err := url.Parse(location)
	if err != nil {
		t.Fatalf("invalid redirect location %q: %v", location, err)
	}
	if u.Scheme != "https" || u.Host != "github.com" || u.Path != "/login/oauth/authorize" {
		t.Fatalf("redirect location = %q", location)
	}
	q := u.Query()
	if q.Get("client_id") != "github-client-id" {
		t.Fatalf("client_id = %q", q.Get("client_id"))
	}
	if q.Get("redirect_uri") != "https://inventory.example.com/auth/github/callback" {
		t.Fatalf("redirect_uri = %q", q.Get("redirect_uri"))
	}
	if q.Get("scope") != "read:user user:email" {
		t.Fatalf("scope = %q", q.Get("scope"))
	}

	state, err := app.decodeOAuthState(stateCookie.Value)
	if err != nil {
		t.Fatalf("decodeOAuthState() error = %v", err)
	}
	if q.Get("state") == "" || q.Get("state") != state.State {
		t.Fatalf("redirect state = %q, cookie state = %q", q.Get("state"), state.State)
	}
}

func TestOAuthCallbackRejectsInvalidRequestsBeforeSessionCreation(t *testing.T) {
	app := testOAuthApp()
	db := newTestAuthDB(t, nil)
	app.db = db
	mux := http.NewServeMux()
	mux.HandleFunc("GET /auth/{provider}/callback", app.HandleOAuthCallback)

	t.Run("missing state cookie", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/auth/github/callback?state=state&code=code", nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	stateCookie, stateValue, err := app.newOAuthStateCookie("github", "/", httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatalf("newOAuthStateCookie() error = %v", err)
	}

	t.Run("state mismatch", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/auth/github/callback?state=wrong&code=code", nil)
		req.AddCookie(stateCookie)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("provider mismatch", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?state="+url.QueryEscape(stateValue)+"&code=code", nil)
		req.AddCookie(stateCookie)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("missing code", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/auth/github/callback?state="+url.QueryEscape(stateValue), nil)
		req.AddCookie(stateCookie)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	if got := testAuthDBConfig(db).execCount.Load(); got != 0 {
		t.Fatalf("callback failure paths executed %d DB statements, want 0", got)
	}
}

func TestUserForSessionValidExpiredMissingAndDisallowed(t *testing.T) {
	token := "raw-session-token"
	tokenHash := hashSessionToken(token)
	db := newTestAuthDB(t, map[string]testSessionRow{
		tokenHash: {
			id:          "user-1",
			email:       "alice@example.com",
			displayName: "Alice",
			avatarURL:   "https://example.com/avatar.png",
			expires:     time.Now().Add(time.Hour),
		},
		hashSessionToken("expired-token"): {
			id:      "user-2",
			email:   "alice@example.com",
			expires: time.Now().Add(-time.Hour),
		},
		hashSessionToken("disallowed-token"): {
			id:      "user-3",
			email:   "mallory@example.net",
			expires: time.Now().Add(time.Hour),
		},
	})
	app := &App{
		db: db,
		auth: &AuthConfig{
			Mode:              authModeOAuth,
			SessionCookieName: "inventorio_session",
			AllowedEmails:     map[string]struct{}{"alice@example.com": {}},
			AllowedDomains:    map[string]struct{}{},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "inventorio_session", Value: token})
	user, err := app.userForSession(req)
	if err != nil {
		t.Fatalf("userForSession() error = %v", err)
	}
	if user.ID != "user-1" || user.Email != "alice@example.com" || user.DisplayName != "Alice" || user.Provider != authModeOAuth {
		t.Fatalf("userForSession() user = %+v", user)
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	if _, err := app.userForSession(req); err == nil {
		t.Fatal("userForSession() without cookie error = nil")
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "inventorio_session", Value: "missing-token"})
	if _, err := app.userForSession(req); err == nil {
		t.Fatal("userForSession() with missing token error = nil")
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "inventorio_session", Value: "expired-token"})
	if _, err := app.userForSession(req); err == nil {
		t.Fatal("userForSession() with expired token error = nil")
	}
	if !testAuthDBConfig(db).deletedToken(hashSessionToken("expired-token")) {
		t.Fatal("expired session token was not deleted")
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "inventorio_session", Value: "disallowed-token"})
	if _, err := app.userForSession(req); err == nil {
		t.Fatal("userForSession() with disallowed user error = nil")
	}
}

func TestLogoutDeletesOAuthSessionAndExpiresCookie(t *testing.T) {
	token := "logout-token"
	db := newTestAuthDB(t, nil)
	app := &App{
		db: db,
		auth: &AuthConfig{
			Mode:              authModeOAuth,
			SessionCookieName: "inventorio_session",
			CookieSecure:      "true",
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	req.AddCookie(&http.Cookie{Name: "inventorio_session", Value: token})
	rr := httptest.NewRecorder()
	app.HandleLogout(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusFound)
	}
	if rr.Header().Get("Location") != "/login" {
		t.Fatalf("Location = %q", rr.Header().Get("Location"))
	}
	if !testAuthDBConfig(db).deletedToken(hashSessionToken(token)) {
		t.Fatal("logout did not delete current session")
	}
	cookie := findCookie(rr.Result().Cookies(), "inventorio_session")
	if cookie == nil {
		t.Fatal("logout did not set expired session cookie")
	}
	if cookie.Value != "" || cookie.MaxAge != -1 {
		t.Fatalf("expired cookie = %+v", cookie)
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

func testOAuthApp() *App {
	return &App{
		auth: &AuthConfig{
			Mode:              authModeOAuth,
			PublicURL:         "https://inventory.example.com",
			SessionSecret:     strongSessionSecret(),
			SessionCookieName: "inventorio_session",
			CookieSecure:      "auto",
			AllowedEmails:     map[string]struct{}{"alice@example.com": {}},
			AllowedDomains:    map[string]struct{}{},
			GitHub: OAuthProviderConfig{
				ClientID:     "github-client-id",
				ClientSecret: "github-client-secret",
			},
			Google: OAuthProviderConfig{
				ClientID:     "google-client-id",
				ClientSecret: "google-client-secret",
			},
		},
	}
}

func findCookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}

type testSessionRow struct {
	id          string
	email       string
	displayName string
	avatarURL   string
	expires     time.Time
}

type testAuthDBState struct {
	sessions  map[string]testSessionRow
	deleted   map[string]bool
	execCount atomic.Int64
	mu        sync.Mutex
}

func (s *testAuthDBState) deletedToken(tokenHash string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deleted[tokenHash]
}

var (
	testAuthDriverOnce sync.Once
	testAuthDBs        sync.Map
	testAuthDBStates   sync.Map
	testAuthDBSeq      atomic.Int64
)

func newTestAuthDB(t *testing.T, sessions map[string]testSessionRow) *sql.DB {
	t.Helper()
	testAuthDriverOnce.Do(func() {
		sql.Register("inventorio_test_auth", testAuthDriver{})
	})
	if sessions == nil {
		sessions = map[string]testSessionRow{}
	}
	name := "db-" + time.Now().Format("20060102150405") + "-" + strconv.FormatInt(testAuthDBSeq.Add(1), 10)
	state := &testAuthDBState{
		sessions: sessions,
		deleted:  map[string]bool{},
	}
	testAuthDBs.Store(name, state)
	t.Cleanup(func() {
		testAuthDBs.Delete(name)
	})
	db, err := sql.Open("inventorio_test_auth", name)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	testAuthDBStates.Store(db, state)
	t.Cleanup(func() {
		testAuthDBStates.Delete(db)
		db.Close()
	})
	return db
}

func testAuthDBConfig(db *sql.DB) *testAuthDBState {
	value, ok := testAuthDBStates.Load(db)
	if !ok {
		return nil
	}
	return value.(*testAuthDBState)
}

type testAuthDriver struct{}

func (testAuthDriver) Open(name string) (driver.Conn, error) {
	value, ok := testAuthDBs.Load(name)
	if !ok {
		return nil, errors.New("unknown test auth database")
	}
	return &testAuthConn{state: value.(*testAuthDBState)}, nil
}

type testAuthConn struct {
	state *testAuthDBState
}

func (c *testAuthConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepared statements are not supported")
}

func (c *testAuthConn) Close() error {
	return nil
}

func (c *testAuthConn) Begin() (driver.Tx, error) {
	return nil, errors.New("transactions are not supported")
}

func (c *testAuthConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if !strings.Contains(query, "FROM sessions s") || len(args) != 1 {
		return &testAuthRows{}, nil
	}
	tokenHash, _ := args[0].Value.(string)
	c.state.mu.Lock()
	row, ok := c.state.sessions[tokenHash]
	c.state.mu.Unlock()
	if !ok {
		return &testAuthRows{}, nil
	}
	return &testAuthRows{
		cols: []string{"id", "email", "display_name", "avatar_url", "expires_at"},
		values: [][]driver.Value{{
			row.id,
			row.email,
			nullableString(row.displayName),
			nullableString(row.avatarURL),
			row.expires,
		}},
	}, nil
}

func (c *testAuthConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	c.state.execCount.Add(1)
	if strings.Contains(query, "DELETE FROM sessions WHERE token_hash = $1") && len(args) == 1 {
		tokenHash, _ := args[0].Value.(string)
		c.state.mu.Lock()
		c.state.deleted[tokenHash] = true
		c.state.mu.Unlock()
	}
	return driver.RowsAffected(1), nil
}

type testAuthRows struct {
	cols   []string
	values [][]driver.Value
	idx    int
}

func (r *testAuthRows) Columns() []string {
	return r.cols
}

func (r *testAuthRows) Close() error {
	return nil
}

func (r *testAuthRows) Next(dest []driver.Value) error {
	if r.idx >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.idx])
	r.idx++
	return nil
}

func nullableString(value string) driver.Value {
	if value == "" {
		return nil
	}
	return value
}
