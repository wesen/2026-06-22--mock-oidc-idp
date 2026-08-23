CREATE TABLE admin_grants (
    id TEXT PRIMARY KEY,
    actor_subject TEXT NOT NULL,
    scope_kind TEXT NOT NULL CHECK (scope_kind IN ('system', 'domain')),
    scope_id TEXT NOT NULL,
    role TEXT NOT NULL,
    capabilities_json BLOB NOT NULL,
    version INTEGER NOT NULL CHECK (version >= 1),
    issued_at_ns INTEGER NOT NULL,
    expires_at_ns INTEGER,
    revoked_at_ns INTEGER
);

CREATE UNIQUE INDEX admin_grants_active_owner_idx
    ON admin_grants(scope_kind, scope_id)
    WHERE role = 'owner' AND revoked_at_ns IS NULL;
CREATE INDEX admin_grants_subject_scope_idx
    ON admin_grants(actor_subject, scope_kind, scope_id);

CREATE TABLE admin_sessions (
    id_hash BLOB PRIMARY KEY,
    actor_subject TEXT NOT NULL,
    grant_id TEXT NOT NULL,
    grant_version INTEGER NOT NULL,
    csrf_hash BLOB NOT NULL,
    authenticated_at_ns INTEGER NOT NULL,
    created_at_ns INTEGER NOT NULL,
    last_seen_at_ns INTEGER NOT NULL,
    expires_at_ns INTEGER NOT NULL,
    revoked_at_ns INTEGER,
    FOREIGN KEY (grant_id) REFERENCES admin_grants(id)
);

CREATE TABLE admin_action_nonces (
    nonce TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    expires_at_ns INTEGER NOT NULL,
    consumed_at_ns INTEGER
);

CREATE TABLE admin_resource_versions (
    resource_type TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    version INTEGER NOT NULL CHECK (version >= 1),
    updated_at_ns INTEGER NOT NULL,
    PRIMARY KEY (resource_type, resource_id)
);

CREATE TABLE admin_idempotency (
    actor_subject TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
    request_hash BLOB NOT NULL,
    status_code INTEGER NOT NULL,
    response_json BLOB NOT NULL,
    created_at_ns INTEGER NOT NULL,
    expires_at_ns INTEGER NOT NULL,
    PRIMARY KEY (actor_subject, idempotency_key)
);

CREATE TABLE admin_actions (
    id TEXT PRIMARY KEY,
    nonce TEXT NOT NULL UNIQUE,
    actor_subject TEXT NOT NULL,
    grant_id TEXT NOT NULL,
    grant_version INTEGER NOT NULL,
    capability TEXT NOT NULL,
    command TEXT NOT NULL,
    target_type TEXT NOT NULL,
    target_id TEXT NOT NULL,
    expected_version INTEGER NOT NULL,
    status TEXT NOT NULL,
    error_code TEXT NOT NULL,
    created_at_ns INTEGER NOT NULL,
    completed_at_ns INTEGER,
    FOREIGN KEY (grant_id) REFERENCES admin_grants(id)
);

CREATE TABLE admin_audit_outbox (
    id TEXT PRIMARY KEY,
    action_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    payload_json BLOB NOT NULL,
    created_at_ns INTEGER NOT NULL,
    delivered_at_ns INTEGER,
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT NOT NULL DEFAULT '',
    FOREIGN KEY (action_id) REFERENCES admin_actions(id)
);
CREATE INDEX admin_audit_outbox_pending_idx
    ON admin_audit_outbox(delivered_at_ns, created_at_ns);

CREATE TABLE admin_auth_attempts (
    state_hash BLOB PRIMARY KEY,
    nonce_hash BLOB NOT NULL,
    pkce_verifier_box BLOB NOT NULL,
    return_path TEXT NOT NULL,
    browser_binding_hash BLOB NOT NULL,
    created_at_ns INTEGER NOT NULL,
    expires_at_ns INTEGER NOT NULL,
    consumed_at_ns INTEGER
);

CREATE TABLE admin_operations (
    id TEXT PRIMARY KEY,
    kind TEXT NOT NULL,
    status TEXT NOT NULL,
    progress_json BLOB NOT NULL,
    result_json BLOB NOT NULL,
    error_code TEXT NOT NULL,
    created_at_ns INTEGER NOT NULL,
    updated_at_ns INTEGER NOT NULL,
    completed_at_ns INTEGER
);

CREATE TABLE admin_invitation_records (
    invitation_id TEXT PRIMARY KEY,
    label TEXT NOT NULL,
    created_by_subject TEXT NOT NULL,
    created_at_ns INTEGER NOT NULL,
    last_issued_at_ns INTEGER
);
