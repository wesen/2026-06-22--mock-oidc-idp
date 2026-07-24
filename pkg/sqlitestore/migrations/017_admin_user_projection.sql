CREATE TABLE admin_user_projection (
    user_id TEXT PRIMARY KEY,
    subject TEXT NOT NULL UNIQUE,
    login TEXT NOT NULL,
    email TEXT NOT NULL,
    display_name TEXT NOT NULL,
    disabled INTEGER NOT NULL CHECK (disabled IN (0, 1)),
    locked_until_ns INTEGER,
    last_successful_login_at_ns INTEGER,
    active_session_count INTEGER NOT NULL DEFAULT 0,
    active_grant_count INTEGER NOT NULL DEFAULT 0,
    created_at_ns INTEGER NOT NULL,
    updated_at_ns INTEGER NOT NULL,
    version INTEGER NOT NULL CHECK (version >= 1)
);
CREATE INDEX admin_user_projection_login_idx ON admin_user_projection(login);
CREATE INDEX admin_user_projection_email_idx ON admin_user_projection(email);
CREATE INDEX admin_user_projection_updated_idx ON admin_user_projection(updated_at_ns DESC, user_id);
