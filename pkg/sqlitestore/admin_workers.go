package sqlitestore

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/go-go-golems/tiny-idp/pkg/idpadminstore"
)

func (s *Store) ListPendingAudit(
	ctx context.Context,
	now time.Time,
	limit int,
) ([]idpadminstore.AuditOutboxRecord, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	rows, err := s.conn().QueryContext(ctx, `
		SELECT id, action_id, event_type, payload_json, created_at_ns,
		       delivered_at_ns, attempts, last_error, next_attempt_at_ns
		FROM admin_audit_outbox
		WHERE delivered_at_ns IS NULL AND next_attempt_at_ns<=?
		ORDER BY next_attempt_at_ns, created_at_ns
		LIMIT ?`, now.UTC().UnixNano(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := make([]idpadminstore.AuditOutboxRecord, 0, limit)
	for rows.Next() {
		var record idpadminstore.AuditOutboxRecord
		var created, next int64
		var delivered sql.NullInt64
		if err := rows.Scan(
			&record.ID, &record.ActionID, &record.EventType, &record.Payload,
			&created, &delivered, &record.Attempts, &record.LastError, &next,
		); err != nil {
			return nil, err
		}
		record.CreatedAt = time.Unix(0, created).UTC()
		record.DeliveredAt = timePointer(delivered)
		record.NextAttemptAt = time.Unix(0, next).UTC()
		records = append(records, record)
	}
	return records, rows.Err()
}

func (s *Store) MarkAuditDelivered(ctx context.Context, id string, deliveredAt time.Time) error {
	result, err := s.conn().ExecContext(ctx, `
		UPDATE admin_audit_outbox
		SET delivered_at_ns=?, attempts=attempts+1, last_error=''
		WHERE id=? AND delivered_at_ns IS NULL`,
		deliveredAt.UTC().UnixNano(), strings.TrimSpace(id))
	return requireOne(result, err, idpadminstore.ErrNotFound)
}

func (s *Store) RecordAuditFailure(
	ctx context.Context,
	id, errorCode string,
	nextAttemptAt time.Time,
) error {
	result, err := s.conn().ExecContext(ctx, `
		UPDATE admin_audit_outbox
		SET attempts=attempts+1, last_error=?, next_attempt_at_ns=?
		WHERE id=? AND delivered_at_ns IS NULL`,
		strings.TrimSpace(errorCode), nextAttemptAt.UTC().UnixNano(), strings.TrimSpace(id))
	return requireOne(result, err, idpadminstore.ErrNotFound)
}

func (s *Store) GetAuditOutboxHealth(ctx context.Context) (idpadminstore.OutboxHealth, error) {
	var health idpadminstore.OutboxHealth
	var oldest sql.NullInt64
	if err := s.conn().QueryRowContext(ctx, `
		SELECT COUNT(*), MIN(created_at_ns)
		FROM admin_audit_outbox
		WHERE delivered_at_ns IS NULL`).Scan(&health.Pending, &oldest); err != nil {
		return idpadminstore.OutboxHealth{}, err
	}
	health.OldestPending = timePointer(oldest)
	_ = s.conn().QueryRowContext(ctx, `
		SELECT last_error FROM admin_audit_outbox
		WHERE delivered_at_ns IS NULL AND last_error<>''
		ORDER BY created_at_ns LIMIT 1`).Scan(&health.LastErrorCode)
	return health, nil
}

func (s *Store) CreateAdminOperation(ctx context.Context, operation idpadminstore.Operation) error {
	if strings.TrimSpace(operation.ID) == "" || strings.TrimSpace(operation.Kind) == "" ||
		strings.TrimSpace(operation.Command) == "" || strings.TrimSpace(operation.ActorSubject) == "" ||
		operation.Status != "pending" || operation.CreatedAt.IsZero() || operation.UpdatedAt.IsZero() {
		return fmt.Errorf("invalid administration operation")
	}
	progress := operation.Progress
	if len(progress) == 0 {
		progress = []byte(`{}`)
	}
	result := operation.Result
	if len(result) == 0 {
		result = []byte(`{}`)
	}
	_, err := s.conn().ExecContext(ctx, `
		INSERT INTO admin_operations
			(id, kind, status, progress_json, result_json, error_code,
			 created_at_ns, updated_at_ns, completed_at_ns,
			 actor_subject, command, label, relative_result_path, started_at_ns)
		VALUES (?, ?, ?, ?, ?, '', ?, ?, NULL, ?, ?, ?, '', NULL)`,
		operation.ID, operation.Kind, operation.Status, progress, result,
		operation.CreatedAt.UTC().UnixNano(), operation.UpdatedAt.UTC().UnixNano(),
		operation.ActorSubject, operation.Command, strings.TrimSpace(operation.Label))
	if isConstraint(err) {
		return idpadminstore.ErrDuplicate
	}
	return err
}

func (s *Store) ClaimAdminOperation(ctx context.Context, id string, startedAt time.Time) (bool, error) {
	result, err := s.conn().ExecContext(ctx, `
		UPDATE admin_operations
		SET status='running', started_at_ns=?, updated_at_ns=?
		WHERE id=? AND status='pending'`,
		startedAt.UTC().UnixNano(), startedAt.UTC().UnixNano(), strings.TrimSpace(id))
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

func (s *Store) ListPendingAdminOperations(ctx context.Context, limit int) ([]idpadminstore.Operation, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	rows, err := s.conn().QueryContext(ctx, `
		SELECT id, kind, command, actor_subject, status, label, progress_json,
		       result_json, relative_result_path, error_code, created_at_ns,
		       updated_at_ns, started_at_ns, completed_at_ns
		FROM admin_operations
		WHERE status='pending'
		ORDER BY created_at_ns, id
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	operations := make([]idpadminstore.Operation, 0, limit)
	for rows.Next() {
		operation, err := scanAdminOperation(rows)
		if err != nil {
			return nil, err
		}
		operations = append(operations, operation)
	}
	return operations, rows.Err()
}

func (s *Store) RequeueRunningAdminOperations(ctx context.Context, updatedAt time.Time) (int64, error) {
	result, err := s.conn().ExecContext(ctx, `
		UPDATE admin_operations
		SET status='pending', started_at_ns=NULL, updated_at_ns=?
		WHERE status='running'`, updatedAt.UTC().UnixNano())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *Store) GetAdminOperation(ctx context.Context, id string) (idpadminstore.Operation, error) {
	row := s.conn().QueryRowContext(ctx, `
		SELECT id, kind, command, actor_subject, status, label, progress_json,
		       result_json, relative_result_path, error_code, created_at_ns,
		       updated_at_ns, started_at_ns, completed_at_ns
		FROM admin_operations
		WHERE id=?`, strings.TrimSpace(id))
	operation, err := scanAdminOperation(row)
	if err == sql.ErrNoRows {
		return idpadminstore.Operation{}, idpadminstore.ErrNotFound
	}
	return operation, err
}

func (s *Store) GetAdminOperationHealth(ctx context.Context) (idpadminstore.OperationHealth, error) {
	var health idpadminstore.OperationHealth
	var oldest sql.NullInt64
	if err := s.conn().QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN status='pending' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status='running' THEN 1 ELSE 0 END), 0),
			MIN(CASE WHEN status IN ('pending','running') THEN created_at_ns END)
		FROM admin_operations`).Scan(&health.Pending, &health.Running, &oldest); err != nil {
		return idpadminstore.OperationHealth{}, err
	}
	health.OldestActive = timePointer(oldest)
	_ = s.conn().QueryRowContext(ctx, `
		SELECT error_code FROM admin_operations
		WHERE status='failed' AND error_code<>''
		ORDER BY updated_at_ns DESC LIMIT 1`).Scan(&health.LastErrorCode)
	return health, nil
}

func (s *Store) CompleteAdminOperation(
	ctx context.Context,
	id string,
	resultJSON []byte,
	relativePath string,
	completedAt time.Time,
) error {
	if len(resultJSON) == 0 {
		resultJSON = []byte(`{}`)
	}
	result, err := s.conn().ExecContext(ctx, `
		UPDATE admin_operations
		SET status='completed', result_json=?, relative_result_path=?,
		    error_code='', updated_at_ns=?, completed_at_ns=?
		WHERE id=? AND status='running'`,
		resultJSON, strings.TrimSpace(relativePath), completedAt.UTC().UnixNano(),
		completedAt.UTC().UnixNano(), strings.TrimSpace(id))
	return requireOne(result, err, idpadminstore.ErrNotFound)
}

func (s *Store) FailAdminOperation(
	ctx context.Context,
	id, errorCode string,
	completedAt time.Time,
) error {
	result, err := s.conn().ExecContext(ctx, `
		UPDATE admin_operations
		SET status='failed', error_code=?, updated_at_ns=?, completed_at_ns=?
		WHERE id=? AND status='running'`,
		strings.TrimSpace(errorCode), completedAt.UTC().UnixNano(),
		completedAt.UTC().UnixNano(), strings.TrimSpace(id))
	return requireOne(result, err, idpadminstore.ErrNotFound)
}

func (s *Store) CreateAdminDownload(ctx context.Context, record idpadminstore.DownloadRecord) error {
	if len(record.HandleHash) < 16 || strings.TrimSpace(record.OperationID) == "" ||
		strings.TrimSpace(record.RelativePath) == "" || strings.TrimSpace(record.ContentType) == "" ||
		strings.TrimSpace(record.DownloadName) == "" || record.CreatedAt.IsZero() ||
		!record.ExpiresAt.After(record.CreatedAt) {
		return fmt.Errorf("invalid administration download")
	}
	_, err := s.conn().ExecContext(ctx, `
		INSERT INTO admin_downloads
			(handle_hash, operation_id, relative_path, content_type, download_name,
			 created_at_ns, expires_at_ns, consumed_at_ns)
		VALUES (?, ?, ?, ?, ?, ?, ?, NULL)`,
		record.HandleHash, record.OperationID, record.RelativePath, record.ContentType,
		record.DownloadName, record.CreatedAt.UTC().UnixNano(), record.ExpiresAt.UTC().UnixNano())
	if isConstraint(err) {
		return idpadminstore.ErrDuplicate
	}
	return err
}

func (s *Store) ConsumeAdminDownload(
	ctx context.Context,
	handleHash []byte,
	now time.Time,
) (idpadminstore.DownloadRecord, error) {
	var record idpadminstore.DownloadRecord
	var created, expires int64
	var consumed sql.NullInt64
	err := s.conn().QueryRowContext(ctx, `
		UPDATE admin_downloads
		SET consumed_at_ns=?
		WHERE handle_hash=? AND consumed_at_ns IS NULL AND expires_at_ns>?
		RETURNING handle_hash, operation_id, relative_path, content_type,
		          download_name, created_at_ns, expires_at_ns, consumed_at_ns`,
		now.UTC().UnixNano(), handleHash, now.UTC().UnixNano()).Scan(
		&record.HandleHash, &record.OperationID, &record.RelativePath,
		&record.ContentType, &record.DownloadName, &created, &expires, &consumed,
	)
	if err == sql.ErrNoRows {
		return idpadminstore.DownloadRecord{}, idpadminstore.ErrNotFound
	}
	if err != nil {
		return idpadminstore.DownloadRecord{}, err
	}
	record.CreatedAt = time.Unix(0, created).UTC()
	record.ExpiresAt = time.Unix(0, expires).UTC()
	record.ConsumedAt = timePointer(consumed)
	return record, nil
}

type operationScanner interface {
	Scan(...any) error
}

func scanAdminOperation(scanner operationScanner) (idpadminstore.Operation, error) {
	var operation idpadminstore.Operation
	var created, updated int64
	var started, completed sql.NullInt64
	if err := scanner.Scan(
		&operation.ID, &operation.Kind, &operation.Command, &operation.ActorSubject,
		&operation.Status, &operation.Label, &operation.Progress, &operation.Result,
		&operation.RelativeResultPath, &operation.ErrorCode, &created, &updated,
		&started, &completed,
	); err != nil {
		return idpadminstore.Operation{}, err
	}
	operation.CreatedAt = time.Unix(0, created).UTC()
	operation.UpdatedAt = time.Unix(0, updated).UTC()
	operation.StartedAt = timePointer(started)
	operation.CompletedAt = timePointer(completed)
	return operation, nil
}
