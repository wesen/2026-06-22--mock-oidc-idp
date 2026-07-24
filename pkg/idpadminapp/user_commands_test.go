package idpadminapp_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/go-go-golems/tiny-idp/pkg/idpaccounts"
	"github.com/go-go-golems/tiny-idp/pkg/idpadmin"
	"github.com/go-go-golems/tiny-idp/pkg/idpadminapp"
	"github.com/go-go-golems/tiny-idp/pkg/idpadminstore"
	"github.com/go-go-golems/tiny-idp/pkg/idpstore"
	"github.com/go-go-golems/tiny-idp/pkg/sqlitestore"
	"github.com/stretchr/testify/require"
)

type userCommandFixture struct {
	ctx       context.Context
	now       time.Time
	store     *sqlitestore.Store
	principal idpadmin.AdminPrincipal
	actions   *idpadminapp.ActionService
	users     *idpadminapp.UserCommandService
}

func TestUserCommandsLifecycleAndStaleAction(t *testing.T) {
	fixture := newUserCommandFixture(t)

	created := fixture.execute(t, idpadminapp.CommandUsersCreate, "", map[string]any{
		"login": "alice", "password": "correct horse battery staple",
		"email": "alice@example.test", "email_verified": true,
		"display_name": "Alice",
	})
	require.Equal(t, int64(1), created.User.Version)
	require.Equal(t, "alice", created.User.Login)
	require.True(t, created.Committed)
	require.Equal(t, "pending", created.AuditStatus)

	first, err := fixture.actions.Prepare(fixture.ctx, fixture.principal, idpadminapp.PrepareActionRequest{
		Command: idpadminapp.CommandUsersUpdate, TargetID: created.User.ID,
	})
	require.NoError(t, err)
	stale, err := fixture.actions.Prepare(fixture.ctx, fixture.principal, idpadminapp.PrepareActionRequest{
		Command: idpadminapp.CommandUsersUpdate, TargetID: created.User.ID,
	})
	require.NoError(t, err)
	updated := fixture.executePrepared(t, first, map[string]any{
		"email": "new@example.test", "email_verified": false,
		"display_name": "Alice Updated", "locale": "en-US",
	})
	require.Equal(t, int64(2), updated.User.Version)
	require.Equal(t, "Alice Updated", updated.User.DisplayName)

	_, err = fixture.executePreparedRaw(stale, map[string]any{
		"email": "stale@example.test", "display_name": "Stale",
	})
	require.ErrorIs(t, err, idpadminstore.ErrVersionConflict)
	afterStale, err := fixture.store.GetAdminUser(fixture.ctx, created.User.ID)
	require.NoError(t, err)
	require.Equal(t, "new@example.test", afterStale.Email)
	require.Equal(t, int64(2), afterStale.Version)

	disabledSession := idpstore.Session{
		IDHash: []byte("disabled-session-hash"), UserID: created.User.ID,
		AuthTime: fixture.now, CreatedAt: fixture.now, LastSeenAt: fixture.now,
		ExpiresAt: fixture.now.Add(time.Hour),
	}
	require.NoError(t, fixture.store.CreateSession(fixture.ctx, disabledSession))
	disabled := fixture.execute(t, idpadminapp.CommandUsersDisable, created.User.ID, map[string]any{
		"reason": "Employment ended", "confirmation": "DISABLE",
	})
	require.True(t, disabled.User.Disabled)
	require.Equal(t, int64(3), disabled.User.Version)
	revokedDisabledSession, err := fixture.store.GetSession(fixture.ctx, disabledSession.IDHash)
	require.NoError(t, err)
	require.NotNil(t, revokedDisabledSession.RevokedAt)

	enabled := fixture.execute(t, idpadminapp.CommandUsersEnable, created.User.ID, map[string]any{
		"reason": "Access approved again",
	})
	require.False(t, enabled.User.Disabled)

	unlocked := fixture.execute(t, idpadminapp.CommandUsersUnlock, created.User.ID, map[string]any{
		"reason": "Identity verified",
	})
	require.Equal(t, int64(5), unlocked.User.Version)

	passwordSession := idpstore.Session{
		IDHash: []byte("password-session-hash"), UserID: created.User.ID,
		AuthTime: fixture.now, CreatedAt: fixture.now, LastSeenAt: fixture.now,
		ExpiresAt: fixture.now.Add(time.Hour),
	}
	require.NoError(t, fixture.store.CreateSession(fixture.ctx, passwordSession))
	password := fixture.execute(t, idpadminapp.CommandUsersSetPassword, created.User.ID, map[string]any{
		"password": "a different correct horse battery staple",
		"reason":   "Owner-requested credential replacement",
	})
	require.Equal(t, int64(6), password.User.Version)
	revokedPasswordSession, err := fixture.store.GetSession(fixture.ctx, passwordSession.IDHash)
	require.NoError(t, err)
	require.NotNil(t, revokedPasswordSession.RevokedAt)

	revoked := fixture.execute(t, idpadminapp.CommandUsersRevokeAccess, created.User.ID, map[string]any{
		"reason": "Security review", "confirmation": "REVOKE",
	})
	require.Equal(t, int64(7), revoked.User.Version)

	var actions, outbox int
	require.NoError(t, fixture.store.SQLDB().QueryRowContext(fixture.ctx, `SELECT COUNT(*) FROM admin_actions`).Scan(&actions))
	require.NoError(t, fixture.store.SQLDB().QueryRowContext(fixture.ctx, `SELECT COUNT(*) FROM admin_audit_outbox`).Scan(&outbox))
	require.Equal(t, 7, actions)
	require.Equal(t, 7, outbox)
}

func TestUserCommandGuardsReasonConfirmationAndFreshAuth(t *testing.T) {
	fixture := newUserCommandFixture(t)
	created := fixture.execute(t, idpadminapp.CommandUsersCreate, "", map[string]any{
		"login": "alice", "password": "correct horse battery staple",
	})

	prepared, err := fixture.actions.Prepare(fixture.ctx, fixture.principal, idpadminapp.PrepareActionRequest{
		Command: idpadminapp.CommandUsersDisable, TargetID: created.User.ID,
	})
	require.NoError(t, err)
	_, err = fixture.executePreparedRaw(prepared, map[string]any{"confirmation": "DISABLE"})
	require.ErrorIs(t, err, idpadminapp.ErrReasonRequired)
	_, err = fixture.executePreparedRaw(prepared, map[string]any{
		"reason": "review", "confirmation": "disable",
	})
	require.ErrorIs(t, err, idpadminapp.ErrConfirmationFailed)

	stalePrincipal := fixture.principal
	stalePrincipal.Authenticated = fixture.now.Add(-10 * time.Minute)
	stalePrincipal.Assurance = idpadmin.AssuranceAuthenticated
	_, err = fixture.actions.Prepare(fixture.ctx, stalePrincipal, idpadminapp.PrepareActionRequest{
		Command: idpadminapp.CommandUsersSetPassword, TargetID: created.User.ID,
	})
	require.ErrorIs(t, err, idpadmin.ErrFreshAuthRequired)
}

func newUserCommandFixture(t *testing.T) *userCommandFixture {
	t.Helper()
	ctx := context.Background()
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	store, err := sqlitestore.Open(ctx, sqlitestore.DefaultConfig(filepath.Join(t.TempDir(), "idp.db")))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	grant := idpadmin.Grant{
		ID: "owner-grant", ActorSubject: "owner-sub", Scope: idpadmin.SystemScope(),
		Role: "owner", Capabilities: idpadmin.AllCapabilities(), Version: 1,
		IssuedAt: now.Add(-time.Hour),
	}
	require.NoError(t, store.CreateAdminGrant(ctx, grant))
	require.NoError(t, func() error {
		_, err := store.RebuildAdminUserProjection(ctx, now)
		return err
	}())
	clock := func() time.Time { return now }
	handles, err := idpadmin.NewHandleService([]byte("0123456789abcdef0123456789abcdef"), time.Minute, clock)
	require.NoError(t, err)
	authorizer, err := idpadmin.NewAuthorizer(store, 5*time.Minute, clock)
	require.NoError(t, err)
	executor, err := idpadminapp.NewExecutor(store, handles, authorizer, clock)
	require.NoError(t, err)
	actions, err := idpadminapp.NewActionService(store, handles, authorizer, clock)
	require.NoError(t, err)
	users, err := idpadminapp.NewUserCommandService(store, executor, idpaccounts.Options{}, clock)
	require.NoError(t, err)
	return &userCommandFixture{
		ctx: ctx, now: now, store: store,
		principal: idpadmin.AdminPrincipal{
			Subject: "owner-sub", SessionID: "session-binding",
			Authenticated: now, Assurance: idpadmin.AssuranceFresh,
			GrantID: grant.ID, GrantVersion: grant.Version,
		},
		actions: actions, users: users,
	}
}

func (f *userCommandFixture) execute(
	t *testing.T,
	command, targetID string,
	input map[string]any,
) idpadmin.UserResult {
	t.Helper()
	prepared, err := f.actions.Prepare(f.ctx, f.principal, idpadminapp.PrepareActionRequest{
		Command: command, TargetID: targetID,
	})
	require.NoError(t, err)
	return f.executePrepared(t, prepared, input)
}

func (f *userCommandFixture) executePrepared(
	t *testing.T,
	prepared idpadminapp.PreparedAction,
	input map[string]any,
) idpadmin.UserResult {
	t.Helper()
	response, err := f.executePreparedRaw(prepared, input)
	require.NoError(t, err)
	var result idpadmin.UserResult
	require.NoError(t, json.Unmarshal(response, &result))
	return result
}

func (f *userCommandFixture) executePreparedRaw(
	prepared idpadminapp.PreparedAction,
	input map[string]any,
) ([]byte, error) {
	raw, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(raw)
	return f.users.Execute(f.ctx, idpadminapp.ExecutionRequest{
		Handle: prepared.Handle, Principal: f.principal,
		RequestID:      fmt.Sprintf("request-%d", time.Now().UnixNano()),
		IdempotencyKey: fmt.Sprintf("idem-%d", time.Now().UnixNano()),
		RequestHash:    sum[:],
	}, raw)
}

func TestUserCommandRejectsReplayWithoutMutation(t *testing.T) {
	fixture := newUserCommandFixture(t)
	created := fixture.execute(t, idpadminapp.CommandUsersCreate, "", map[string]any{
		"login": "alice", "password": "correct horse battery staple",
	})
	prepared, err := fixture.actions.Prepare(fixture.ctx, fixture.principal, idpadminapp.PrepareActionRequest{
		Command: idpadminapp.CommandUsersUpdate, TargetID: created.User.ID,
	})
	require.NoError(t, err)
	raw, err := json.Marshal(map[string]any{"display_name": "First"})
	require.NoError(t, err)
	sum := sha256.Sum256(raw)
	request := idpadminapp.ExecutionRequest{
		Handle: prepared.Handle, Principal: fixture.principal,
		RequestID: "replay-request", IdempotencyKey: "replay-idempotency",
		RequestHash: sum[:],
	}
	firstInput := append([]byte(nil), raw...)
	_, err = fixture.users.Execute(fixture.ctx, request, firstInput)
	require.NoError(t, err)
	secondInput := append([]byte(nil), raw...)
	response, err := fixture.users.Execute(fixture.ctx, request, secondInput)
	require.NoError(t, err)
	require.NotEmpty(t, response)
	row, err := fixture.store.GetAdminUser(fixture.ctx, created.User.ID)
	require.NoError(t, err)
	require.Equal(t, int64(2), row.Version)

	conflicting := append([]byte(nil), []byte(`{"display_name":"Second"}`)...)
	otherHash := sha256.Sum256(conflicting)
	request.RequestHash = otherHash[:]
	_, err = fixture.users.Execute(fixture.ctx, request, conflicting)
	require.True(t, errors.Is(err, idpadminstore.ErrIdempotencyConflict))
}

func TestUserCommandConcurrentSameKeyMutatesOnce(t *testing.T) {
	fixture := newUserCommandFixture(t)
	created := fixture.execute(t, idpadminapp.CommandUsersCreate, "", map[string]any{
		"login": "alice", "password": "correct horse battery staple",
	})
	prepared, err := fixture.actions.Prepare(fixture.ctx, fixture.principal, idpadminapp.PrepareActionRequest{
		Command: idpadminapp.CommandUsersUpdate, TargetID: created.User.ID,
	})
	require.NoError(t, err)
	raw := []byte(`{"display_name":"Concurrent"}`)
	sum := sha256.Sum256(raw)
	request := idpadminapp.ExecutionRequest{
		Handle: prepared.Handle, Principal: fixture.principal,
		RequestID: "concurrent-request", IdempotencyKey: "concurrent-key",
		RequestHash: sum[:],
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			input := append([]byte(nil), raw...)
			_, executeErr := fixture.users.Execute(fixture.ctx, request, input)
			results <- executeErr
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	for result := range results {
		require.NoError(t, result)
	}
	row, err := fixture.store.GetAdminUser(fixture.ctx, created.User.ID)
	require.NoError(t, err)
	require.Equal(t, int64(2), row.Version)
	var actions int
	require.NoError(t, fixture.store.SQLDB().QueryRowContext(fixture.ctx,
		`SELECT COUNT(*) FROM admin_actions WHERE request_id='concurrent-request'`).Scan(&actions))
	require.Equal(t, 1, actions)
}

var _ idpstore.Store = (*sqlitestore.Store)(nil)
