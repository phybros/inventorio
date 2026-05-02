package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const oauthStateCookieName = "inventorio_oauth_state"

type oauthState struct {
	Provider string `json:"provider"`
	State    string `json:"state"`
	Next     string `json:"next"`
	Expires  int64  `json:"expires"`
}

type oauthTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Error       string `json:"error"`
	Description string `json:"error_description"`
}

func (app *App) oauthStartURL(provider, state string) (string, error) {
	cfg := app.auth.Provider(provider)
	callback := app.auth.PublicURL + "/auth/" + provider + "/callback"
	values := url.Values{}
	values.Set("client_id", cfg.ClientID)
	values.Set("redirect_uri", callback)
	values.Set("response_type", "code")
	values.Set("state", state)

	switch provider {
	case "github":
		values.Set("scope", "read:user user:email")
		return "https://github.com/login/oauth/authorize?" + values.Encode(), nil
	case "google":
		values.Set("scope", "openid email profile")
		return "https://accounts.google.com/o/oauth2/v2/auth?" + values.Encode(), nil
	default:
		return "", fmt.Errorf("unknown oauth provider %q", provider)
	}
}

func (app *App) newOAuthStateCookie(provider, next string, r *http.Request) (*http.Cookie, string, error) {
	stateValue, err := randomToken(32)
	if err != nil {
		return nil, "", err
	}
	state := oauthState{
		Provider: provider,
		State:    stateValue,
		Next:     sanitizeNext(next),
		Expires:  time.Now().Add(10 * time.Minute).Unix(),
	}
	encoded, err := app.encodeOAuthState(state)
	if err != nil {
		return nil, "", err
	}
	return &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    encoded,
		Path:     "/auth/" + provider,
		Expires:  time.Unix(state.Expires, 0),
		MaxAge:   600,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   app.sessionCookieSecure(r),
	}, stateValue, nil
}

func (app *App) expiredOAuthStateCookie(provider string, r *http.Request) *http.Cookie {
	return &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    "",
		Path:     "/auth/" + provider,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   app.sessionCookieSecure(r),
	}
}

func (app *App) encodeOAuthState(state oauthState) (string, error) {
	payload, err := json.Marshal(state)
	if err != nil {
		return "", err
	}
	payload64 := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(app.auth.SessionSecret))
	mac.Write([]byte(payload64))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payload64 + "." + sig, nil
}

func (app *App) decodeOAuthState(raw string) (oauthState, error) {
	var state oauthState
	parts := strings.Split(raw, ".")
	if len(parts) != 2 {
		return state, errors.New("invalid oauth state cookie")
	}
	mac := hmac.New(sha256.New, []byte(app.auth.SessionSecret))
	mac.Write([]byte(parts[0]))
	want := mac.Sum(nil)
	got, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(got, want) {
		return state, errors.New("invalid oauth state signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return state, err
	}
	if err := json.Unmarshal(payload, &state); err != nil {
		return state, err
	}
	if time.Now().Unix() > state.Expires {
		return state, errors.New("oauth state expired")
	}
	return state, nil
}

func (app *App) fetchOAuthIdentity(ctx context.Context, provider, code string) (AuthenticatedIdentity, error) {
	switch provider {
	case "github":
		return app.fetchGitHubIdentity(ctx, code)
	case "google":
		return app.fetchGoogleIdentity(ctx, code)
	default:
		return AuthenticatedIdentity{}, fmt.Errorf("unknown oauth provider %q", provider)
	}
}

func (app *App) exchangeOAuthCode(ctx context.Context, provider, code string) (string, error) {
	cfg := app.auth.Provider(provider)
	values := url.Values{}
	values.Set("client_id", cfg.ClientID)
	values.Set("client_secret", cfg.ClientSecret)
	values.Set("code", code)
	values.Set("redirect_uri", app.auth.PublicURL+"/auth/"+provider+"/callback")

	endpoint := ""
	if provider == "github" {
		endpoint = "https://github.com/login/oauth/access_token"
	} else {
		endpoint = "https://oauth2.googleapis.com/token"
		values.Set("grant_type", "authorization_code")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		io.Copy(io.Discard, resp.Body)
		return "", fmt.Errorf("oauth token exchange failed with status %d", resp.StatusCode)
	}

	var token oauthTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return "", err
	}
	if token.Error != "" {
		return "", fmt.Errorf("oauth token exchange failed: %s", token.Error)
	}
	if token.AccessToken == "" {
		return "", errors.New("oauth token exchange returned no access token")
	}
	return token.AccessToken, nil
}

func (app *App) fetchGitHubIdentity(ctx context.Context, code string) (AuthenticatedIdentity, error) {
	token, err := app.exchangeOAuthCode(ctx, "github", code)
	if err != nil {
		return AuthenticatedIdentity{}, err
	}

	type githubUser struct {
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url"`
	}
	var user githubUser
	if err := fetchJSON(ctx, "https://api.github.com/user", token, &user); err != nil {
		return AuthenticatedIdentity{}, err
	}

	type githubEmail struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	var emails []githubEmail
	if err := fetchJSON(ctx, "https://api.github.com/user/emails", token, &emails); err != nil {
		return AuthenticatedIdentity{}, err
	}

	canonical := ""
	for _, email := range emails {
		if email.Primary && email.Verified {
			canonical = email.Email
			break
		}
	}
	if canonical == "" {
		return AuthenticatedIdentity{}, errors.New("github account has no primary verified email")
	}
	normalized, err := normalizeEmail(canonical)
	if err != nil {
		return AuthenticatedIdentity{}, err
	}
	displayName := user.Name
	if displayName == "" {
		displayName = user.Login
	}
	return AuthenticatedIdentity{
		Provider:        "github",
		ProviderSubject: fmt.Sprintf("%d", user.ID),
		ProviderLogin:   user.Login,
		Email:           normalized,
		DisplayName:     displayName,
		AvatarURL:       user.AvatarURL,
	}, nil
}

func (app *App) fetchGoogleIdentity(ctx context.Context, code string) (AuthenticatedIdentity, error) {
	token, err := app.exchangeOAuthCode(ctx, "google", code)
	if err != nil {
		return AuthenticatedIdentity{}, err
	}

	type googleUser struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
	}
	var user googleUser
	if err := fetchJSON(ctx, "https://openidconnect.googleapis.com/v1/userinfo", token, &user); err != nil {
		return AuthenticatedIdentity{}, err
	}
	if !user.EmailVerified {
		return AuthenticatedIdentity{}, errors.New("google account email is not verified")
	}
	normalized, err := normalizeEmail(user.Email)
	if err != nil {
		return AuthenticatedIdentity{}, err
	}
	return AuthenticatedIdentity{
		Provider:        "google",
		ProviderSubject: user.Sub,
		ProviderLogin:   normalized,
		Email:           normalized,
		DisplayName:     user.Name,
		AvatarURL:       user.Picture,
	}, nil
}

func fetchJSON(ctx context.Context, endpoint, token string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		io.Copy(io.Discard, resp.Body)
		return fmt.Errorf("oauth profile fetch failed with status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(dest)
}
