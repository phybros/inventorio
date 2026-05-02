package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"
)

const sessionLifetime = 30 * 24 * time.Hour

func hashSessionToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (app *App) createSession(ctx context.Context, userID string) (string, time.Time, error) {
	token, err := randomToken(32)
	if err != nil {
		return "", time.Time{}, err
	}
	expires := time.Now().Add(sessionLifetime)
	_, err = app.db.ExecContext(ctx, `
		INSERT INTO sessions (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, userID, hashSessionToken(token), expires)
	if err != nil {
		return "", time.Time{}, err
	}
	return token, expires, nil
}

func (app *App) userForSession(r *http.Request) (*CurrentUser, error) {
	cookie, err := r.Cookie(app.auth.SessionCookieName)
	if err != nil || cookie.Value == "" {
		return nil, fmt.Errorf("missing session cookie")
	}
	tokenHash := hashSessionToken(cookie.Value)

	var user CurrentUser
	var displayName, avatarURL sql.NullString
	var expires time.Time
	err = app.db.QueryRowContext(r.Context(), `
		SELECT u.id, u.email, u.display_name, u.avatar_url, s.expires_at
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1
	`, tokenHash).Scan(&user.ID, &user.Email, &displayName, &avatarURL, &expires)
	if err != nil {
		return nil, err
	}
	if time.Now().After(expires) {
		_, _ = app.db.ExecContext(r.Context(), `DELETE FROM sessions WHERE token_hash = $1`, tokenHash)
		return nil, fmt.Errorf("session expired")
	}
	if !app.auth.EmailAllowed(user.Email) {
		return nil, fmt.Errorf("session user is no longer allowed")
	}
	if displayName.Valid {
		user.DisplayName = displayName.String
	}
	if avatarURL.Valid {
		user.AvatarURL = avatarURL.String
	}
	user.Provider = authModeOAuth

	_, _ = app.db.ExecContext(r.Context(), `
		UPDATE sessions SET last_seen_at = now()
		WHERE token_hash = $1 AND last_seen_at < now() - interval '5 minutes'
	`, tokenHash)
	_, _ = app.db.ExecContext(r.Context(), `
		UPDATE users SET last_seen_at = now()
		WHERE id = $1 AND last_seen_at < now() - interval '5 minutes'
	`, user.ID)

	return &user, nil
}

func (app *App) deleteCurrentSession(r *http.Request) {
	cookie, err := r.Cookie(app.auth.SessionCookieName)
	if err != nil || cookie.Value == "" {
		return
	}
	_, _ = app.db.ExecContext(r.Context(), `DELETE FROM sessions WHERE token_hash = $1`, hashSessionToken(cookie.Value))
}
