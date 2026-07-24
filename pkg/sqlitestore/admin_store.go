package sqlitestore

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-go-golems/tiny-idp/pkg/idpadmin"
	"github.com/go-go-golems/tiny-idp/pkg/idpadminstore"
	"github.com/go-go-golems/tiny-idp/pkg/idpstore"
)

var _ idpadminstore.Store = (*Store)(nil)

func (s *Store) AdminUpdate(ctx context.Context, fn func(idpstore.TxStore, idpadminstore.TxStore) error) error {
	if fn == nil {
		return fmt.Errorf("admin update callback is required")
	}
	if s.runner != nil {
		return idpstore.ErrNestedTransaction
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin admin write transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	scoped := &Store{db: s.db, runner: tx, mu: s.mu, path: s.path, backupCopy: s.backupCopy}
	if err := fn(scoped, scoped); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit admin write transaction: %w", err)
	}
	return nil
}

func (s *Store) CreateAdminGrant(ctx context.Context, grant idpadmin.Grant) error {
	if err := grant.Validate(); err != nil {
		return err
	}
	capabilities, err := enc(grant.Capabilities)
	if err != nil {
		return fmt.Errorf("encode grant capabilities: %w", err)
	}
	_, err = s.conn().ExecContext(ctx, `
		INSERT INTO admin_grants
			(id, actor_subject, scope_kind, scope_id, role, capabilities_json, version, issued_at_ns, expires_at_ns, revoked_at_ns)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		grant.ID, grant.ActorSubject, grant.Scope.Kind, grant.Scope.ID, grant.Role, capabilities,
		grant.Version, grant.IssuedAt.UnixNano(), nullableTime(grant.ExpiresAt), nullableTime(grant.RevokedAt))
	if isConstraint(err) {
		return idpadminstore.ErrDuplicate
	}
	return err
}

func (s *Store) GetAdminGrant(ctx context.Context, id string) (idpadmin.Grant, error) {
	var grant idpadmin.Grant
	var kind string
	var capabilities []byte
	var issued int64
	var expires, revoked sql.NullInt64
	err := s.conn().QueryRowContext(ctx, `
		SELECT id, actor_subject, scope_kind, scope_id, role, capabilities_json, version,
		       issued_at_ns, expires_at_ns, revoked_at_ns
		FROM admin_grants WHERE id=?`, id).
		Scan(&grant.ID, &grant.ActorSubject, &kind, &grant.Scope.ID, &grant.Role, &capabilities,
			&grant.Version, &issued, &expires, &revoked)
	if errors.Is(err, sql.ErrNoRows) {
		return idpadmin.Grant{}, idpadmin.ErrGrantNotFound
	}
	if err != nil {
		return idpadmin.Grant{}, err
	}
	grant.Scope.Kind = idpadmin.ScopeKind(kind)
	grant.IssuedAt = time.Unix(0, issued).UTC()
	grant.ExpiresAt = timePointer(expires)
	grant.RevokedAt = timePointer(revoked)
	if grant.Capabilities, err = dec[[]idpadmin.Capability](capabilities); err != nil {
		return idpadmin.Grant{}, fmt.Errorf("decode grant capabilities: %w", err)
	}
	return grant, nil
}

func (s *Store) FindActiveAdminGrant(ctx context.Context, subject string, scope idpadmin.AdminScope, now time.Time) (idpadmin.Grant, error) {
	var id string
	err := s.conn().QueryRowContext(ctx, `
		SELECT id FROM admin_grants
		WHERE actor_subject=? AND scope_kind=? AND scope_id=? AND revoked_at_ns IS NULL
		  AND (expires_at_ns IS NULL OR expires_at_ns>?)
		ORDER BY issued_at_ns DESC LIMIT 1`,
		subject, scope.Kind, scope.ID, now.UnixNano()).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return idpadmin.Grant{}, idpadmin.ErrGrantNotFound
	}
	if err != nil {
		return idpadmin.Grant{}, err
	}
	return s.GetAdminGrant(ctx, id)
}

func (s *Store) GetActiveSystemOwner(ctx context.Context, now time.Time) (idpadmin.Grant, error) {
	var id string
	err := s.conn().QueryRowContext(ctx, `
		SELECT id FROM admin_grants
		WHERE scope_kind='system' AND scope_id='system' AND role='owner'
		  AND revoked_at_ns IS NULL AND (expires_at_ns IS NULL OR expires_at_ns>?)
		ORDER BY issued_at_ns DESC LIMIT 1`, now.UnixNano()).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return idpadmin.Grant{}, idpadmin.ErrGrantNotFound
	}
	if err != nil {
		return idpadmin.Grant{}, err
	}
	return s.GetAdminGrant(ctx, id)
}

func (s *Store) RevokeAdminGrant(ctx context.Context, id string, expectedVersion int64, at time.Time) error {
	result, err := s.conn().ExecContext(ctx, `
		UPDATE admin_grants SET revoked_at_ns=?, version=version+1
		WHERE id=? AND version=? AND revoked_at_ns IS NULL`, at.UnixNano(), id, expectedVersion)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 1 {
		return nil
	}
	var exists int
	if err := s.conn().QueryRowContext(ctx, `SELECT 1 FROM admin_grants WHERE id=?`, id).Scan(&exists); errors.Is(err, sql.ErrNoRows) {
		return idpadmin.ErrGrantNotFound
	}
	return idpadminstore.ErrVersionConflict
}

func (s *Store) CreateAdminSession(ctx context.Context, session idpadminstore.Session) error {
	_, err := s.conn().ExecContext(ctx, `
		INSERT INTO admin_sessions
			(id_hash, actor_subject, grant_id, grant_version, csrf_hash, authenticated_at_ns,
			 created_at_ns, last_seen_at_ns, expires_at_ns, revoked_at_ns)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		session.IDHash, session.Subject, session.GrantID, session.GrantVersion, session.CSRFHash,
		session.AuthenticatedAt.UnixNano(), session.CreatedAt.UnixNano(), session.LastSeenAt.UnixNano(),
		session.ExpiresAt.UnixNano(), nullableTime(session.RevokedAt))
	if isConstraint(err) {
		return idpadminstore.ErrDuplicate
	}
	return err
}

func (s *Store) GetAdminSession(ctx context.Context, idHash []byte) (idpadminstore.Session, error) {
	var session idpadminstore.Session
	var authenticated, created, seen, expires int64
	var revoked sql.NullInt64
	err := s.conn().QueryRowContext(ctx, `
		SELECT id_hash, actor_subject, grant_id, grant_version, csrf_hash, authenticated_at_ns,
		       created_at_ns, last_seen_at_ns, expires_at_ns, revoked_at_ns
		FROM admin_sessions WHERE id_hash=?`, idHash).
		Scan(&session.IDHash, &session.Subject, &session.GrantID, &session.GrantVersion, &session.CSRFHash,
			&authenticated, &created, &seen, &expires, &revoked)
	if errors.Is(err, sql.ErrNoRows) {
		return idpadminstore.Session{}, idpadminstore.ErrNotFound
	}
	if err != nil {
		return idpadminstore.Session{}, err
	}
	session.AuthenticatedAt = time.Unix(0, authenticated).UTC()
	session.CreatedAt = time.Unix(0, created).UTC()
	session.LastSeenAt = time.Unix(0, seen).UTC()
	session.ExpiresAt = time.Unix(0, expires).UTC()
	session.RevokedAt = timePointer(revoked)
	return session, nil
}

func (s *Store) RevokeAdminSession(ctx context.Context, idHash []byte, at time.Time) error {
	result, err := s.conn().ExecContext(ctx, `
		UPDATE admin_sessions SET revoked_at_ns=? WHERE id_hash=? AND revoked_at_ns IS NULL`,
		at.UnixNano(), idHash)
	return requireOne(result, err, idpadminstore.ErrNotFound)
}

func (s *Store) TouchAdminSession(ctx context.Context, idHash []byte, seenAt time.Time) error {
	result, err := s.conn().ExecContext(ctx, `
		UPDATE admin_sessions SET last_seen_at_ns=?
		WHERE id_hash=? AND revoked_at_ns IS NULL AND expires_at_ns>?`,
		seenAt.UnixNano(), idHash, seenAt.UnixNano())
	return requireOne(result, err, idpadminstore.ErrNotFound)
}

func (s *Store) RotateAdminSessionCSRF(ctx context.Context, idHash, csrfHash []byte, now time.Time) error {
	result, err := s.conn().ExecContext(ctx, `
		UPDATE admin_sessions SET csrf_hash=?, last_seen_at_ns=?
		WHERE id_hash=? AND revoked_at_ns IS NULL AND expires_at_ns>?`,
		csrfHash, now.UnixNano(), idHash, now.UnixNano())
	return requireOne(result, err, idpadminstore.ErrNotFound)
}

func (s *Store) CreateAdminAuthAttempt(ctx context.Context, attempt idpadminstore.AuthAttempt) error {
	_, err := s.conn().ExecContext(ctx, `
		INSERT INTO admin_auth_attempts
			(state_hash, nonce_hash, pkce_verifier_box, return_path, browser_binding_hash,
			 created_at_ns, expires_at_ns, consumed_at_ns)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		attempt.StateHash, attempt.NonceHash, attempt.PKCEVerifierBox, attempt.ReturnPath,
		attempt.BrowserBindingHash, attempt.CreatedAt.UnixNano(), attempt.ExpiresAt.UnixNano(),
		nullableTime(attempt.ConsumedAt))
	if isConstraint(err) {
		return idpadminstore.ErrDuplicate
	}
	return err
}

func (s *Store) ConsumeAdminAuthAttempt(
	ctx context.Context,
	stateHash, browserBindingHash []byte,
	now time.Time,
) (idpadminstore.AuthAttempt, error) {
	if s.runner == nil {
		var result idpadminstore.AuthAttempt
		err := s.AdminUpdate(ctx, func(_ idpstore.TxStore, admin idpadminstore.TxStore) error {
			var err error
			result, err = admin.ConsumeAdminAuthAttempt(ctx, stateHash, browserBindingHash, now)
			return err
		})
		return result, err
	}
	var attempt idpadminstore.AuthAttempt
	var created, expires int64
	var consumed sql.NullInt64
	err := s.conn().QueryRowContext(ctx, `
		SELECT state_hash, nonce_hash, pkce_verifier_box, return_path, browser_binding_hash,
		       created_at_ns, expires_at_ns, consumed_at_ns
		FROM admin_auth_attempts WHERE state_hash=?`, stateHash).
		Scan(&attempt.StateHash, &attempt.NonceHash, &attempt.PKCEVerifierBox, &attempt.ReturnPath,
			&attempt.BrowserBindingHash, &created, &expires, &consumed)
	if errors.Is(err, sql.ErrNoRows) {
		return idpadminstore.AuthAttempt{}, idpadminstore.ErrNotFound
	}
	if err != nil {
		return idpadminstore.AuthAttempt{}, err
	}
	if consumed.Valid || expires <= now.UnixNano() ||
		subtle.ConstantTimeCompare(attempt.BrowserBindingHash, browserBindingHash) != 1 {
		return idpadminstore.AuthAttempt{}, idpadminstore.ErrNotFound
	}
	result, err := s.conn().ExecContext(ctx, `
		UPDATE admin_auth_attempts SET consumed_at_ns=?
		WHERE state_hash=? AND consumed_at_ns IS NULL AND expires_at_ns>?`,
		now.UnixNano(), stateHash, now.UnixNano())
	if err := requireOne(result, err, idpadminstore.ErrNotFound); err != nil {
		return idpadminstore.AuthAttempt{}, err
	}
	attempt.CreatedAt = time.Unix(0, created).UTC()
	attempt.ExpiresAt = time.Unix(0, expires).UTC()
	attempt.ConsumedAt = &now
	return attempt, nil
}

func (s *Store) CreateActionNonce(ctx context.Context, nonce, sessionID string, expiresAt time.Time) error {
	_, err := s.conn().ExecContext(ctx, `
		INSERT INTO admin_action_nonces(nonce, session_id, expires_at_ns) VALUES (?, ?, ?)`,
		actionNonceKey(nonce), sessionID, expiresAt.UnixNano())
	if isConstraint(err) {
		return idpadminstore.ErrDuplicate
	}
	return err
}

func (s *Store) ConsumeActionNonce(ctx context.Context, nonce, sessionID string, now time.Time) error {
	result, err := s.conn().ExecContext(ctx, `
		UPDATE admin_action_nonces SET consumed_at_ns=?
		WHERE nonce=? AND session_id=? AND consumed_at_ns IS NULL AND expires_at_ns>?`,
		now.UnixNano(), actionNonceKey(nonce), sessionID, now.UnixNano())
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 1 {
		return nil
	}
	return idpadminstore.ErrNonceConsumed
}

func (s *Store) GetResourceVersion(ctx context.Context, resourceType, resourceID string) (int64, error) {
	var version int64
	err := s.conn().QueryRowContext(ctx, `
		SELECT version FROM admin_resource_versions WHERE resource_type=? AND resource_id=?`,
		resourceType, resourceID).Scan(&version)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, idpadminstore.ErrNotFound
	}
	return version, err
}

func (s *Store) CreateResourceVersion(ctx context.Context, resourceType, resourceID string, version int64, now time.Time) error {
	if version < 1 {
		return fmt.Errorf("resource version must be positive")
	}
	_, err := s.conn().ExecContext(ctx, `
		INSERT INTO admin_resource_versions(resource_type, resource_id, version, updated_at_ns)
		VALUES (?, ?, ?, ?)`, resourceType, resourceID, version, now.UTC().UnixNano())
	if isConstraint(err) {
		return idpadminstore.ErrDuplicate
	}
	return err
}

func (s *Store) CompareAndIncrementResourceVersion(ctx context.Context, resourceType, resourceID string, expected int64, now time.Time) (int64, error) {
	result, err := s.conn().ExecContext(ctx, `
		UPDATE admin_resource_versions SET version=version+1, updated_at_ns=?
		WHERE resource_type=? AND resource_id=? AND version=?`,
		now.UnixNano(), resourceType, resourceID, expected)
	if err != nil {
		return 0, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if count != 1 {
		return 0, idpadminstore.ErrVersionConflict
	}
	return expected + 1, nil
}

func (s *Store) CreateAdminInvitation(ctx context.Context, record idpadminstore.InvitationRecord) error {
	if strings.TrimSpace(record.InvitationID) == "" ||
		strings.TrimSpace(record.CreatedBySubject) == "" ||
		record.CreatedAt.IsZero() || record.LastIssuedAt.IsZero() {
		return fmt.Errorf("invalid administration invitation record")
	}
	_, err := s.conn().ExecContext(ctx, `
		INSERT INTO admin_invitation_records
			(invitation_id, label, created_by_subject, created_at_ns, last_issued_at_ns)
		VALUES (?, ?, ?, ?, ?)`,
		record.InvitationID, strings.TrimSpace(record.Label), record.CreatedBySubject,
		record.CreatedAt.UTC().UnixNano(), record.LastIssuedAt.UTC().UnixNano())
	if err != nil {
		if isConstraint(err) {
			return idpadminstore.ErrDuplicate
		}
		return err
	}
	return nil
}

func (s *Store) PutIdempotencyRecord(ctx context.Context, record idpadminstore.IdempotencyRecord) error {
	_, err := s.conn().ExecContext(ctx, `
		INSERT INTO admin_idempotency
			(actor_subject, idempotency_key, request_hash, status_code, response_json, created_at_ns, expires_at_ns)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		record.Subject, record.Key, record.RequestHash, record.StatusCode, record.Response,
		record.CreatedAt.UnixNano(), record.ExpiresAt.UnixNano())
	if isConstraint(err) {
		return idpadminstore.ErrIdempotencyConflict
	}
	return err
}

func (s *Store) GetIdempotencyRecord(ctx context.Context, subject, key string) (idpadminstore.IdempotencyRecord, error) {
	var record idpadminstore.IdempotencyRecord
	var created, expires int64
	err := s.conn().QueryRowContext(ctx, `
		SELECT actor_subject, idempotency_key, request_hash, status_code, response_json, created_at_ns, expires_at_ns
		FROM admin_idempotency WHERE actor_subject=? AND idempotency_key=?`, subject, key).
		Scan(&record.Subject, &record.Key, &record.RequestHash, &record.StatusCode, &record.Response, &created, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return idpadminstore.IdempotencyRecord{}, idpadminstore.ErrNotFound
	}
	if err != nil {
		return idpadminstore.IdempotencyRecord{}, err
	}
	record.CreatedAt = time.Unix(0, created).UTC()
	record.ExpiresAt = time.Unix(0, expires).UTC()
	return record, nil
}

func (s *Store) InsertAdminAction(ctx context.Context, action idpadminstore.Action) error {
	_, err := s.conn().ExecContext(ctx, `
		INSERT INTO admin_actions
			(id, nonce, actor_subject, grant_id, grant_version, capability, command, target_type,
			 target_id, expected_version, status, error_code, created_at_ns, completed_at_ns,
			 request_id, session_binding, scope_kind, scope_id, resulting_version,
			 operator_reason, assurance)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		action.ID, actionNonceKey(action.Nonce), action.Subject, action.GrantID, action.GrantVersion, action.Capability,
		action.Command, action.TargetType, action.TargetID, action.ExpectedVersion, action.Status,
		action.ErrorCode, action.CreatedAt.UnixNano(), nullableTime(action.CompletedAt),
		action.RequestID, action.SessionBinding, action.Scope.Kind, action.Scope.ID,
		action.ResultingVersion, action.Reason, action.Assurance)
	if isConstraint(err) {
		return idpadminstore.ErrDuplicate
	}
	return err
}

func (s *Store) EnqueueAudit(ctx context.Context, record idpadminstore.AuditOutboxRecord) error {
	_, err := s.conn().ExecContext(ctx, `
		INSERT INTO admin_audit_outbox
			(id, action_id, event_type, payload_json, created_at_ns, delivered_at_ns,
			 attempts, last_error, next_attempt_at_ns)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.ID, record.ActionID, record.EventType, record.Payload, record.CreatedAt.UnixNano(),
		nullableTime(record.DeliveredAt), record.Attempts, record.LastError,
		firstNonzeroTime(record.NextAttemptAt, record.CreatedAt).UnixNano())
	if isConstraint(err) {
		return idpadminstore.ErrDuplicate
	}
	return err
}

func firstNonzeroTime(value, fallback time.Time) time.Time {
	if value.IsZero() {
		return fallback.UTC()
	}
	return value.UTC()
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UnixNano()
}

func actionNonceKey(nonce string) string {
	sum := sha256.Sum256([]byte(nonce))
	return hex.EncodeToString(sum[:])
}

func timePointer(value sql.NullInt64) *time.Time {
	if !value.Valid {
		return nil
	}
	t := time.Unix(0, value.Int64).UTC()
	return &t
}

func isConstraint(err error) bool {
	return err != nil && (errors.Is(err, idpstore.ErrDuplicate) ||
		containsAny(err.Error(), "UNIQUE constraint failed", "PRIMARY KEY constraint failed"))
}

func containsAny(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if len(value) >= len(candidate) {
			for i := 0; i+len(candidate) <= len(value); i++ {
				if value[i:i+len(candidate)] == candidate {
					return true
				}
			}
		}
	}
	return false
}

func requireOne(result sql.Result, err error, notFound error) error {
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return notFound
	}
	return nil
}
