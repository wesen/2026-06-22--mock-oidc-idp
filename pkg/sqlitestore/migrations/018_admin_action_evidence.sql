ALTER TABLE admin_actions ADD COLUMN request_id TEXT NOT NULL DEFAULT '';
ALTER TABLE admin_actions ADD COLUMN session_binding TEXT NOT NULL DEFAULT '';
ALTER TABLE admin_actions ADD COLUMN scope_kind TEXT NOT NULL DEFAULT 'system';
ALTER TABLE admin_actions ADD COLUMN scope_id TEXT NOT NULL DEFAULT 'system';
ALTER TABLE admin_actions ADD COLUMN resulting_version INTEGER NOT NULL DEFAULT 0;
ALTER TABLE admin_actions ADD COLUMN operator_reason TEXT NOT NULL DEFAULT '';
ALTER TABLE admin_actions ADD COLUMN assurance TEXT NOT NULL DEFAULT 'authenticated';

CREATE INDEX admin_actions_target_created_idx
    ON admin_actions(target_type, target_id, created_at_ns DESC);
