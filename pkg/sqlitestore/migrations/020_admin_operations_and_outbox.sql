ALTER TABLE admin_audit_outbox ADD COLUMN next_attempt_at_ns INTEGER;

UPDATE admin_audit_outbox
SET next_attempt_at_ns = created_at_ns
WHERE next_attempt_at_ns IS NULL;

CREATE UNIQUE INDEX admin_audit_outbox_action_idx
    ON admin_audit_outbox(action_id);

CREATE INDEX admin_audit_outbox_retry_idx
    ON admin_audit_outbox(delivered_at_ns, next_attempt_at_ns, created_at_ns);

ALTER TABLE admin_operations ADD COLUMN actor_subject TEXT NOT NULL DEFAULT '';
ALTER TABLE admin_operations ADD COLUMN command TEXT NOT NULL DEFAULT '';
ALTER TABLE admin_operations ADD COLUMN label TEXT NOT NULL DEFAULT '';
ALTER TABLE admin_operations ADD COLUMN relative_result_path TEXT NOT NULL DEFAULT '';
ALTER TABLE admin_operations ADD COLUMN started_at_ns INTEGER;

CREATE INDEX admin_operations_pending_idx
    ON admin_operations(status, created_at_ns);
