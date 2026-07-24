CREATE TABLE admin_downloads (
    handle_hash BLOB PRIMARY KEY,
    operation_id TEXT NOT NULL,
    relative_path TEXT NOT NULL,
    content_type TEXT NOT NULL,
    download_name TEXT NOT NULL,
    created_at_ns INTEGER NOT NULL,
    expires_at_ns INTEGER NOT NULL,
    consumed_at_ns INTEGER,
    FOREIGN KEY (operation_id) REFERENCES admin_operations(id)
);

CREATE INDEX admin_downloads_expiry_idx
    ON admin_downloads(consumed_at_ns, expires_at_ns);
