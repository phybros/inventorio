package main

import (
	"log"
	"net/http"
)

type loginPageData struct {
	Next             string
	GitHubConfigured bool
	GoogleConfigured bool
	Message          string
}

func (app *App) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if app.auth.Mode == authModeDisabled {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	next := sanitizeNext(r.URL.Query().Get("next"))
	data := loginPageData{
		Next:             next,
		GitHubConfigured: app.auth.ProviderConfigured("github"),
		GoogleConfigured: app.auth.ProviderConfigured("google"),
	}
	if app.auth.Mode == authModeProxy {
		data.Message = "Authentication is managed by the reverse proxy."
	}
	app.renderer.RenderPage(w, r, "auth/login", data)
}

func (app *App) HandleOAuthStart(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")
	if app.auth.Mode != authModeOAuth || !app.auth.ProviderConfigured(provider) {
		http.NotFound(w, r)
		return
	}
	stateCookie, state, err := app.newOAuthStateCookie(provider, r.URL.Query().Get("next"), r)
	if err != nil {
		log.Printf("failed to create oauth state: %v", err)
		http.Error(w, "failed to start login", http.StatusInternalServerError)
		return
	}
	startURL, err := app.oauthStartURL(provider, state)
	if err != nil {
		http.Error(w, "failed to start login", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, stateCookie)
	http.Redirect(w, r, startURL, http.StatusFound)
}

func (app *App) HandleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")
	if app.auth.Mode != authModeOAuth || !app.auth.ProviderConfigured(provider) {
		http.NotFound(w, r)
		return
	}

	stateCookie, err := r.Cookie(oauthStateCookieName)
	if err != nil {
		http.Error(w, "missing OAuth state", http.StatusBadRequest)
		return
	}
	state, err := app.decodeOAuthState(stateCookie.Value)
	if err != nil || state.Provider != provider || state.State != r.URL.Query().Get("state") {
		http.Error(w, "invalid OAuth state", http.StatusBadRequest)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing OAuth code", http.StatusBadRequest)
		return
	}

	identity, err := app.fetchOAuthIdentity(r.Context(), provider, code)
	if err != nil {
		log.Printf("oauth identity fetch failed for %s: %v", provider, err)
		http.Error(w, "OAuth login failed", http.StatusUnauthorized)
		return
	}
	if !app.auth.EmailAllowed(identity.Email) {
		http.Error(w, "account is not allowed", http.StatusForbidden)
		return
	}

	user, err := app.upsertAuthenticatedUser(r.Context(), identity)
	if err != nil {
		log.Printf("oauth user upsert failed: %v", err)
		http.Error(w, "OAuth login failed", http.StatusInternalServerError)
		return
	}
	token, expires, err := app.createSession(r.Context(), user.ID)
	if err != nil {
		log.Printf("session creation failed: %v", err)
		http.Error(w, "OAuth login failed", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, app.expiredOAuthStateCookie(provider, r))
	http.SetCookie(w, app.sessionCookie(token, expires, r))
	http.Redirect(w, r, sanitizeNext(state.Next), http.StatusFound)
}

func (app *App) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if app.auth.Mode == authModeOAuth {
		app.deleteCurrentSession(r)
		http.SetCookie(w, app.expiredSessionCookie(r))
	}
	http.Redirect(w, r, "/login", http.StatusFound)
}
