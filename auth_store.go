package main

import (
	"context"
	"database/sql"
)

func (app *App) upsertAuthenticatedUser(ctx context.Context, identity AuthenticatedIdentity) (*CurrentUser, error) {
	email, err := normalizeEmail(identity.Email)
	if err != nil {
		return nil, err
	}

	tx, err := app.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var user CurrentUser
	var displayName, avatarURL sql.NullString
	err = tx.QueryRowContext(ctx, `
		SELECT u.id, u.email, u.display_name, u.avatar_url
		FROM user_identities ui
		JOIN users u ON u.id = ui.user_id
		WHERE ui.provider = $1 AND ui.provider_subject = $2
	`, identity.Provider, identity.ProviderSubject).Scan(&user.ID, &user.Email, &displayName, &avatarURL)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	if err == sql.ErrNoRows {
		err = tx.QueryRowContext(ctx, `
			INSERT INTO users (email, display_name, avatar_url)
			VALUES ($1, NULLIF($2, ''), NULLIF($3, ''))
			ON CONFLICT (email) DO UPDATE SET
				display_name = COALESCE(NULLIF(EXCLUDED.display_name, ''), users.display_name),
				avatar_url = COALESCE(NULLIF(EXCLUDED.avatar_url, ''), users.avatar_url),
				updated_at = now(),
				last_seen_at = now()
			RETURNING id, email, display_name, avatar_url
		`, email, identity.DisplayName, identity.AvatarURL).Scan(&user.ID, &user.Email, &displayName, &avatarURL)
		if err != nil {
			return nil, err
		}
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO user_identities (user_id, provider, provider_subject, provider_login)
		VALUES ($1, $2, $3, NULLIF($4, ''))
		ON CONFLICT (provider, provider_subject) DO UPDATE SET
			provider_login = COALESCE(NULLIF(EXCLUDED.provider_login, ''), user_identities.provider_login),
			updated_at = now(),
			last_used_at = now()
	`, user.ID, identity.Provider, identity.ProviderSubject, identity.ProviderLogin)
	if err != nil {
		return nil, err
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE users SET
			display_name = COALESCE(NULLIF($2, ''), display_name),
			avatar_url = COALESCE(NULLIF($3, ''), avatar_url),
			updated_at = now(),
			last_seen_at = now()
		WHERE id = $1
	`, user.ID, identity.DisplayName, identity.AvatarURL)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	if identity.DisplayName != "" {
		displayName = sql.NullString{String: identity.DisplayName, Valid: true}
	}
	if identity.AvatarURL != "" {
		avatarURL = sql.NullString{String: identity.AvatarURL, Valid: true}
	}
	if displayName.Valid {
		user.DisplayName = displayName.String
	}
	if avatarURL.Valid {
		user.AvatarURL = avatarURL.String
	}
	user.Provider = identity.Provider
	return &user, nil
}
