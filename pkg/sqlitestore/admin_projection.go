package sqlitestore

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"sort"
	"time"

	"github.com/go-go-golems/tiny-idp/pkg/idpadminstore"
	"github.com/go-go-golems/tiny-idp/pkg/idpstore"
)

type adminUserProjection struct {
	UserID                string
	Subject               string
	Login                 string
	Email                 string
	DisplayName           string
	Disabled              bool
	LockedUntil           *time.Time
	LastSuccessfulLoginAt *time.Time
	ActiveSessionCount    int
	ActiveGrantCount      int
	CreatedAt             time.Time
	UpdatedAt             time.Time
	Version               int64
}

func (s *Store) RebuildAdminUserProjection(ctx context.Context, now time.Time) (idpadminstore.ProjectionReport, error) {
	var report idpadminstore.ProjectionReport
	err := s.AdminUpdate(ctx, func(_ idpstore.TxStore, admin idpadminstore.TxStore) error {
		scoped, ok := admin.(*Store)
		if !ok {
			return fmt.Errorf("unexpected admin transaction implementation")
		}
		expected, err := scoped.expectedAdminUserProjection(ctx, now, true)
		if err != nil {
			return err
		}
		if _, err := scoped.conn().ExecContext(ctx, `DELETE FROM admin_user_projection`); err != nil {
			return fmt.Errorf("clear admin user projection: %w", err)
		}
		for _, row := range expected {
			if err := scoped.putAdminUserProjection(ctx, row); err != nil {
				return err
			}
		}
		report = idpadminstore.ProjectionReport{
			SourceRows: len(expected), ProjectionRows: len(expected),
		}
		return nil
	})
	return report, err
}

func (s *Store) CheckAdminUserProjection(ctx context.Context, now time.Time) (idpadminstore.ProjectionReport, error) {
	expected, err := s.expectedAdminUserProjection(ctx, now, false)
	if err != nil {
		return idpadminstore.ProjectionReport{}, err
	}
	actual, err := s.readAdminUserProjection(ctx)
	if err != nil {
		return idpadminstore.ProjectionReport{}, err
	}
	report := idpadminstore.ProjectionReport{SourceRows: len(expected), ProjectionRows: len(actual)}
	expectedByID := make(map[string]adminUserProjection, len(expected))
	for _, row := range expected {
		expectedByID[row.UserID] = row
	}
	for _, row := range actual {
		want, ok := expectedByID[row.UserID]
		if !ok || !reflect.DeepEqual(want, row) {
			report.Mismatches++
		}
		delete(expectedByID, row.UserID)
	}
	report.Mismatches += len(expectedByID)
	return report, nil
}

func (s *Store) RefreshAdminUserProjection(ctx context.Context, userID string, now time.Time) error {
	if s.runner == nil {
		return fmt.Errorf("admin user projection refresh requires a transaction")
	}
	expected, err := s.expectedAdminUserProjection(ctx, now, false)
	if err != nil {
		return err
	}
	for _, row := range expected {
		if row.UserID == userID {
			return s.putAdminUserProjection(ctx, row)
		}
	}
	return idpstore.ErrNotFound
}

func (s *Store) expectedAdminUserProjection(ctx context.Context, now time.Time, initializeVersions bool) ([]adminUserProjection, error) {
	users, err := s.readProjectionUsers(ctx)
	if err != nil {
		return nil, err
	}
	sessionCounts, err := s.activeSessionCounts(ctx, now)
	if err != nil {
		return nil, err
	}
	grantCounts, err := s.activeGrantCounts(ctx, now)
	if err != nil {
		return nil, err
	}
	rows := make([]adminUserProjection, 0, len(users))
	for _, source := range users {
		security, err := s.projectionSecurityState(ctx, source.user.ID)
		if err != nil {
			return nil, err
		}
		if initializeVersions {
			if _, err := s.conn().ExecContext(ctx, `
				INSERT INTO admin_resource_versions(resource_type, resource_id, version, updated_at_ns)
				VALUES ('user', ?, 1, ?)
				ON CONFLICT(resource_type, resource_id) DO NOTHING`, source.user.ID, now.UnixNano()); err != nil {
				return nil, fmt.Errorf("initialize user resource version: %w", err)
			}
		}
		version, err := s.GetResourceVersion(ctx, "user", source.user.ID)
		if err == idpadminstore.ErrNotFound && !initializeVersions {
			version = 0
			err = nil
		}
		if err != nil {
			return nil, err
		}
		rows = append(rows, adminUserProjection{
			UserID: source.user.ID, Subject: source.user.Sub, Login: source.login,
			Email: source.user.Email, DisplayName: source.user.Name, Disabled: source.user.Disabled,
			LockedUntil: security.LockedUntil, LastSuccessfulLoginAt: security.LastSuccessfulLoginAt,
			ActiveSessionCount: sessionCounts[source.user.ID], ActiveGrantCount: grantCounts[source.user.ID],
			CreatedAt: source.user.CreatedAt, UpdatedAt: source.user.UpdatedAt, Version: version,
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].UserID < rows[j].UserID })
	return rows, nil
}

type projectionUserSource struct {
	login string
	user  idpstore.User
}

func (s *Store) readProjectionUsers(ctx context.Context) ([]projectionUserSource, error) {
	rows, err := s.conn().QueryContext(ctx, `SELECT login, data FROM users ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list users for admin projection: %w", err)
	}
	defer rows.Close()
	var result []projectionUserSource
	for rows.Next() {
		var login string
		var data []byte
		if err := rows.Scan(&login, &data); err != nil {
			return nil, err
		}
		user, err := dec[idpstore.User](data)
		if err != nil {
			return nil, fmt.Errorf("decode user projection source: %w", err)
		}
		result = append(result, projectionUserSource{login: login, user: user})
	}
	return result, rows.Err()
}

func (s *Store) projectionSecurityState(ctx context.Context, userID string) (idpstore.AccountSecurityState, error) {
	var data []byte
	err := s.conn().QueryRowContext(ctx, `SELECT data FROM account_security_states WHERE user_id=?`, userID).Scan(&data)
	if err == sql.ErrNoRows {
		return idpstore.AccountSecurityState{UserID: userID}, nil
	}
	if err != nil {
		return idpstore.AccountSecurityState{}, err
	}
	return dec[idpstore.AccountSecurityState](data)
}

func (s *Store) activeSessionCounts(ctx context.Context, now time.Time) (map[string]int, error) {
	return countActiveRecords[idpstore.Session](ctx, s, "sessions", func(session idpstore.Session) (string, bool) {
		return session.UserID, session.RevokedAt == nil && now.Before(session.ExpiresAt)
	})
}

func (s *Store) activeGrantCounts(ctx context.Context, now time.Time) (map[string]int, error) {
	return countActiveRecords[idpstore.Grant](ctx, s, "grants", func(grant idpstore.Grant) (string, bool) {
		return grant.UserID, grant.RevokedAt == nil && now.Before(grant.ExpiresAt)
	})
}

func countActiveRecords[T any](
	ctx context.Context,
	store *Store,
	table string,
	classify func(T) (string, bool),
) (map[string]int, error) {
	rows, err := store.conn().QueryContext(ctx, `SELECT data FROM `+table) //nolint:gosec // table is a closed internal constant.
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := map[string]int{}
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		record, err := dec[T](data)
		if err != nil {
			return nil, err
		}
		id, active := classify(record)
		if active {
			counts[id]++
		}
	}
	return counts, rows.Err()
}

func (s *Store) putAdminUserProjection(ctx context.Context, row adminUserProjection) error {
	_, err := s.conn().ExecContext(ctx, `
		INSERT INTO admin_user_projection
			(user_id, subject, login, email, display_name, disabled, locked_until_ns,
			 last_successful_login_at_ns, active_session_count, active_grant_count,
			 created_at_ns, updated_at_ns, version)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			subject=excluded.subject,
			login=excluded.login,
			email=excluded.email,
			display_name=excluded.display_name,
			disabled=excluded.disabled,
			locked_until_ns=excluded.locked_until_ns,
			last_successful_login_at_ns=excluded.last_successful_login_at_ns,
			active_session_count=excluded.active_session_count,
			active_grant_count=excluded.active_grant_count,
			created_at_ns=excluded.created_at_ns,
			updated_at_ns=excluded.updated_at_ns,
			version=excluded.version`,
		row.UserID, row.Subject, row.Login, row.Email, row.DisplayName, row.Disabled,
		nullableTime(row.LockedUntil), nullableTime(row.LastSuccessfulLoginAt),
		row.ActiveSessionCount, row.ActiveGrantCount, row.CreatedAt.UnixNano(),
		row.UpdatedAt.UnixNano(), row.Version)
	if err != nil {
		return fmt.Errorf("insert admin user projection: %w", err)
	}
	return nil
}

func (s *Store) readAdminUserProjection(ctx context.Context) ([]adminUserProjection, error) {
	rows, err := s.conn().QueryContext(ctx, `
		SELECT user_id, subject, login, email, display_name, disabled, locked_until_ns,
		       last_successful_login_at_ns, active_session_count, active_grant_count,
		       created_at_ns, updated_at_ns, version
		FROM admin_user_projection ORDER BY user_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []adminUserProjection
	for rows.Next() {
		var row adminUserProjection
		var disabled bool
		var locked, login sql.NullInt64
		var created, updated int64
		if err := rows.Scan(
			&row.UserID, &row.Subject, &row.Login, &row.Email, &row.DisplayName, &disabled,
			&locked, &login, &row.ActiveSessionCount, &row.ActiveGrantCount,
			&created, &updated, &row.Version,
		); err != nil {
			return nil, err
		}
		row.Disabled = disabled
		row.LockedUntil = timePointer(locked)
		row.LastSuccessfulLoginAt = timePointer(login)
		row.CreatedAt = time.Unix(0, created).UTC()
		row.UpdatedAt = time.Unix(0, updated).UTC()
		result = append(result, row)
	}
	return result, rows.Err()
}
