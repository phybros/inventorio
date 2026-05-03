DROP INDEX IF EXISTS idx_audit_log_actor_user_id;
DROP INDEX IF EXISTS idx_sessions_expires_at;
DROP INDEX IF EXISTS idx_sessions_user_id;
DROP INDEX IF EXISTS idx_user_identities_user_id;

ALTER TABLE audit_log
DROP COLUMN IF EXISTS actor_email,
DROP COLUMN IF EXISTS actor_user_id;

DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS user_identities;
DROP TABLE IF EXISTS users;
