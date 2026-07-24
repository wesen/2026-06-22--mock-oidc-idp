package sqlitestore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-go-golems/tiny-idp/pkg/idpadmin"
	"github.com/go-go-golems/tiny-idp/pkg/idpadminstore"
	"github.com/go-go-golems/tiny-idp/pkg/idpstore"
)

func (s *Store) GetAdminOverview(ctx context.Context, now time.Time) (idpadmin.Overview, error) {
	var overview idpadmin.Overview
	err := s.conn().QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(disabled), 0)
		FROM admin_user_projection`).Scan(&overview.UserCount, &overview.DisabledUserCount)
	if err != nil {
		return idpadmin.Overview{}, err
	}
	clients, err := s.ListClients(ctx)
	if err != nil {
		return idpadmin.Overview{}, err
	}
	for _, client := range clients {
		if !client.Disabled {
			overview.ActiveClientCount++
		}
	}
	if err := s.conn().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM admin_operations WHERE status IN ('pending', 'running')`).
		Scan(&overview.PendingOperationCount); err != nil {
		return idpadmin.Overview{}, err
	}
	if err := s.conn().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM durable_invitations
		WHERE revoked_at_ns IS NULL AND redeemed_at_ns IS NULL AND expires_at_ns>?`,
		now.UnixNano()).Scan(&overview.PendingInvitationCount); err != nil {
		return idpadmin.Overview{}, err
	}
	overview.SchemaVersion, err = s.SchemaVersion(ctx)
	return overview, err
}

func (s *Store) GetAdminUser(ctx context.Context, userID string) (idpadmin.UserRow, error) {
	var row idpadmin.UserRow
	var locked, lastLogin sql.NullInt64
	var created, updated int64
	err := s.conn().QueryRowContext(ctx, `
		SELECT user_id, subject, login, email, display_name, disabled, locked_until_ns,
		       last_successful_login_at_ns, active_session_count, active_grant_count,
		       created_at_ns, updated_at_ns, version
		FROM admin_user_projection WHERE user_id=?`, userID).Scan(
		&row.ID, &row.Subject, &row.Login, &row.Email, &row.DisplayName, &row.Disabled,
		&locked, &lastLogin, &row.ActiveSessionCount, &row.ActiveGrantCount,
		&created, &updated, &row.Version,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return idpadmin.UserRow{}, idpadminstore.ErrNotFound
	}
	if err != nil {
		return idpadmin.UserRow{}, err
	}
	row.LockedUntil = timePointer(locked)
	row.LastSuccessfulLoginAt = timePointer(lastLogin)
	row.CreatedAt = time.Unix(0, created).UTC()
	row.UpdatedAt = time.Unix(0, updated).UTC()
	return row, nil
}

func (s *Store) ListAdminUsers(ctx context.Context, filter idpadmin.UserFilter, limit int) ([]idpadmin.UserRow, error) {
	page := (idpadmin.CursorPageRequest{Limit: limit}).Normalized()
	query := strings.TrimSpace(filter.Query)
	if len(query) > idpadmin.MaxSearchLength {
		return nil, fmt.Errorf("user search exceeds %d characters", idpadmin.MaxSearchLength)
	}
	pattern := "%" + strings.ToLower(query) + "%"
	rows, err := s.conn().QueryContext(ctx, `
		SELECT user_id, subject, login, email, display_name, disabled, locked_until_ns,
		       last_successful_login_at_ns, active_session_count, active_grant_count,
		       created_at_ns, updated_at_ns, version
		FROM admin_user_projection
		WHERE (?='' OR lower(login) LIKE ? OR lower(email) LIKE ? OR lower(display_name) LIKE ?)
		  AND (?='' OR (?='active' AND disabled=0 AND locked_until_ns IS NULL)
		       OR (?='disabled' AND disabled=1)
		       OR (?='locked' AND locked_until_ns IS NOT NULL))
		ORDER BY updated_at_ns DESC, user_id DESC
		LIMIT ?`,
		query, pattern, pattern, pattern, filter.Status, filter.Status, filter.Status, filter.Status, page.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]idpadmin.UserRow, 0, page.Limit)
	for rows.Next() {
		var row idpadmin.UserRow
		var locked, login sql.NullInt64
		var created, updated int64
		if err := rows.Scan(
			&row.ID, &row.Subject, &row.Login, &row.Email, &row.DisplayName, &row.Disabled,
			&locked, &login, &row.ActiveSessionCount, &row.ActiveGrantCount,
			&created, &updated, &row.Version,
		); err != nil {
			return nil, err
		}
		row.LockedUntil = timePointer(locked)
		row.LastSuccessfulLoginAt = timePointer(login)
		row.CreatedAt = time.Unix(0, created).UTC()
		row.UpdatedAt = time.Unix(0, updated).UTC()
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s *Store) ListAdminActivity(ctx context.Context, limit int) ([]idpadmin.ActivityRow, error) {
	page := (idpadmin.CursorPageRequest{Limit: limit}).Normalized()
	rows, err := s.conn().QueryContext(ctx, `
		SELECT id, actor_subject, command, target_type, target_id, status, error_code, created_at_ns
		FROM admin_actions ORDER BY created_at_ns DESC, id DESC LIMIT ?`, page.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]idpadmin.ActivityRow, 0, page.Limit)
	for rows.Next() {
		var row idpadmin.ActivityRow
		var created int64
		if err := rows.Scan(
			&row.ID, &row.Subject, &row.Command, &row.TargetType, &row.TargetID,
			&row.Result, &row.ErrorCode, &created,
		); err != nil {
			return nil, err
		}
		row.EventType = row.Command
		row.CreatedAt = time.Unix(0, created).UTC()
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s *Store) ListAdminOperations(ctx context.Context, limit int) (idpadmin.OperationsView, error) {
	page := (idpadmin.CursorPageRequest{Limit: limit}).Normalized()
	rows, err := s.conn().QueryContext(ctx, `
		SELECT id, kind, status, error_code, created_at_ns, updated_at_ns, completed_at_ns
		FROM admin_operations ORDER BY updated_at_ns DESC, id DESC LIMIT ?`, page.Limit)
	if err != nil {
		return idpadmin.OperationsView{}, err
	}
	defer rows.Close()
	view := idpadmin.OperationsView{Operations: make([]idpadmin.OperationRow, 0, page.Limit)}
	for rows.Next() {
		var row idpadmin.OperationRow
		var created, updated int64
		var completed sql.NullInt64
		if err := rows.Scan(
			&row.ID, &row.Kind, &row.Status, &row.ErrorCode,
			&created, &updated, &completed,
		); err != nil {
			return idpadmin.OperationsView{}, err
		}
		row.CreatedAt = time.Unix(0, created).UTC()
		row.UpdatedAt = time.Unix(0, updated).UTC()
		row.CompletedAt = timePointer(completed)
		view.Operations = append(view.Operations, row)
	}
	if err := rows.Err(); err != nil {
		return idpadmin.OperationsView{}, err
	}
	if err := s.conn().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM admin_audit_outbox WHERE delivered_at_ns IS NULL`).Scan(&view.PendingOutbox); err != nil {
		return idpadmin.OperationsView{}, err
	}
	_ = s.conn().QueryRowContext(ctx, `
		SELECT last_error FROM admin_audit_outbox
		WHERE last_error<>'' ORDER BY created_at_ns DESC LIMIT 1`).Scan(&view.LastAuditError)
	return view, nil
}

func (s *Store) ListAdminInvitations(ctx context.Context, now time.Time, limit int) ([]idpadmin.InvitationRow, error) {
	page := (idpadmin.CursorPageRequest{Limit: limit}).Normalized()
	type metadata struct {
		label     string
		createdAt time.Time
	}
	metadataByID := map[string]metadata{}
	metadataRows, err := s.conn().QueryContext(ctx, `
		SELECT invitation_id, label, created_at_ns FROM admin_invitation_records`)
	if err != nil {
		return nil, err
	}
	for metadataRows.Next() {
		var id, label string
		var created int64
		if err := metadataRows.Scan(&id, &label, &created); err != nil {
			_ = metadataRows.Close()
			return nil, err
		}
		metadataByID[id] = metadata{label: label, createdAt: time.Unix(0, created).UTC()}
	}
	if err := metadataRows.Err(); err != nil {
		_ = metadataRows.Close()
		return nil, err
	}
	if err := metadataRows.Close(); err != nil {
		return nil, err
	}
	rows, err := s.conn().QueryContext(ctx, `
		SELECT data FROM durable_invitations ORDER BY expires_at_ns DESC LIMIT ?`, page.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]idpadmin.InvitationRow, 0, page.Limit)
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		invitation, err := dec[idpstore.DurableInvitation](data)
		if err != nil {
			return nil, err
		}
		status := "pending"
		switch {
		case invitation.RevokedAt != nil:
			status = "revoked"
		case invitation.RedeemedAt != nil:
			status = "redeemed"
		case !now.Before(invitation.ExpiresAt):
			status = "expired"
		}
		row := idpadmin.InvitationRow{
			ID: invitation.ID, Audience: invitation.Audience, Status: status,
			ExpiresAt: invitation.ExpiresAt, RevokedAt: invitation.RevokedAt,
			RedeemedAt: invitation.RedeemedAt,
		}
		version, versionErr := s.GetResourceVersion(ctx, "invitation", invitation.ID)
		if versionErr == nil {
			row.Version = version
		} else if !errors.Is(versionErr, idpadminstore.ErrNotFound) {
			return nil, versionErr
		}
		if value, ok := metadataByID[invitation.ID]; ok {
			row.Label = value.label
			row.CreatedAt = value.createdAt
		}
		result = append(result, row)
	}
	return result, rows.Err()
}
