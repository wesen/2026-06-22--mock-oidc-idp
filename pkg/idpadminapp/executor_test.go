package idpadminapp_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/go-go-golems/tiny-idp/pkg/idpadmin"
	"github.com/go-go-golems/tiny-idp/pkg/idpadminapp"
	"github.com/go-go-golems/tiny-idp/pkg/idpadminstore"
	"github.com/go-go-golems/tiny-idp/pkg/idpstore"
	"github.com/go-go-golems/tiny-idp/pkg/sqlitestore"
	"github.com/stretchr/testify/require"
)

func TestExecutorCommitsMutationEvidenceAndIdempotencyAtomically(t *testing.T) {
	ctx := context.Background()
	store := openAdminStore(t)
	now := time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC)
	grant := executorGrant(now)
	require.NoError(t, store.CreateAdminGrant(ctx, grant))
	require.NoError(t, seedResourceVersion(ctx, store, "user", "user-1", 4, now))
	handles, _, executor := newExecutorFixture(t, store, &now)
	principal := executorPrincipal(now)
	raw, err := handles.Mint(idpadmin.ActionClaims{
		SessionID: principal.SessionID, Subject: principal.Subject, GrantID: grant.ID,
		GrantVersion: grant.Version, Scope: grant.Scope, Capability: idpadmin.CapabilityUsersWrite,
		Command: "users.update", TargetType: "user", TargetID: "user-1", ExpectedVersion: 4,
	})
	require.NoError(t, err)
	claims, err := handles.Verify(raw, principal)
	require.NoError(t, err)
	require.NoError(t, store.CreateActionNonce(ctx, claims.Nonce, claims.SessionID, claims.ExpiresAt))
	requestHash := sha256.Sum256([]byte(`{"name":"Alice"}`))
	calls := 0
	mutation := func(ctx context.Context, protocol idpstore.TxStore, _ idpadminstore.TxStore, _ idpadmin.ActionClaims) ([]byte, error) {
		calls++
		require.NoError(t, protocol.PutUser(ctx, "alice", idpstore.User{ID: "user-1", Sub: "subject-1", Name: "Alice"}))
		return []byte(`{"status":"updated"}`), nil
	}
	request := idpadminapp.ExecutionRequest{
		Handle: raw, Principal: principal, IdempotencyKey: "request-1", RequestHash: requestHash[:],
	}

	response, err := executor.Execute(ctx, request, mutation)
	require.NoError(t, err)
	require.JSONEq(t, `{"status":"updated"}`, string(response))
	require.Equal(t, 1, calls)
	version, err := store.GetResourceVersion(ctx, "user", "user-1")
	require.NoError(t, err)
	require.Equal(t, int64(5), version)
	assertTableCount(t, store, "admin_actions", 1)
	assertTableCount(t, store, "admin_audit_outbox", 1)

	response, err = executor.Execute(ctx, request, mutation)
	require.NoError(t, err)
	require.JSONEq(t, `{"status":"updated"}`, string(response))
	require.Equal(t, 1, calls)
	otherHash := sha256.Sum256([]byte(`{"name":"Mallory"}`))
	request.RequestHash = otherHash[:]
	_, err = executor.Execute(ctx, request, mutation)
	require.ErrorIs(t, err, idpadminstore.ErrIdempotencyConflict)
}

func TestExecutorRollbackPreservesNonceWhenCASIsStale(t *testing.T) {
	ctx := context.Background()
	store := openAdminStore(t)
	now := time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC)
	grant := executorGrant(now)
	require.NoError(t, store.CreateAdminGrant(ctx, grant))
	require.NoError(t, seedResourceVersion(ctx, store, "user", "user-1", 2, now))
	handles, _, executor := newExecutorFixture(t, store, &now)
	principal := executorPrincipal(now)
	raw, err := handles.Mint(idpadmin.ActionClaims{
		SessionID: principal.SessionID, Subject: principal.Subject, GrantID: grant.ID,
		GrantVersion: grant.Version, Scope: grant.Scope, Capability: idpadmin.CapabilityUsersWrite,
		Command: "users.disable", TargetType: "user", TargetID: "user-1", ExpectedVersion: 1,
	})
	require.NoError(t, err)
	claims, err := handles.Verify(raw, principal)
	require.NoError(t, err)
	require.NoError(t, store.CreateActionNonce(ctx, claims.Nonce, claims.SessionID, claims.ExpiresAt))
	requestHash := sha256.Sum256([]byte(`{"disabled":true}`))
	request := idpadminapp.ExecutionRequest{
		Handle: raw, Principal: principal, IdempotencyKey: "request-2", RequestHash: requestHash[:],
	}
	mutated := false
	_, err = executor.Execute(ctx, request, func(context.Context, idpstore.TxStore, idpadminstore.TxStore, idpadmin.ActionClaims) ([]byte, error) {
		mutated = true
		return nil, nil
	})
	require.ErrorIs(t, err, idpadminstore.ErrVersionConflict)
	require.False(t, mutated)
	require.NoError(t, store.ConsumeActionNonce(ctx, claims.Nonce, claims.SessionID, now))
	assertTableCount(t, store, "admin_actions", 0)
}

func TestExecutorNeverReplaysOneTimeSecret(t *testing.T) {
	ctx := context.Background()
	store := openAdminStore(t)
	now := time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC)
	grant := executorGrant(now)
	require.NoError(t, store.CreateAdminGrant(ctx, grant))
	handles, _, executor := newExecutorFixture(t, store, &now)
	principal := executorPrincipal(now)
	raw, err := handles.Mint(idpadmin.ActionClaims{
		SessionID: principal.SessionID, Subject: principal.Subject, GrantID: grant.ID,
		GrantVersion: grant.Version, Scope: grant.Scope, Capability: idpadmin.CapabilityClientSecretRotate,
		Command: "clients.rotate_secret", TargetType: "client", TargetID: "app-1",
	})
	require.NoError(t, err)
	claims, err := handles.Verify(raw, principal)
	require.NoError(t, err)
	require.NoError(t, store.CreateActionNonce(ctx, claims.Nonce, claims.SessionID, claims.ExpiresAt))
	requestHash := sha256.Sum256([]byte(`{"rotate":true}`))
	request := idpadminapp.ExecutionRequest{
		Handle: raw, Principal: principal, IdempotencyKey: "secret-1", RequestHash: requestHash[:],
		Policy: idpadminapp.ExecutionPolicy{SecretBearing: true},
	}
	response, err := executor.Execute(ctx, request, func(context.Context, idpstore.TxStore, idpadminstore.TxStore, idpadmin.ActionClaims) ([]byte, error) {
		return []byte(`{"secret":"only-once"}`), nil
	})
	require.NoError(t, err)
	require.Contains(t, string(response), "only-once")
	_, err = executor.Execute(ctx, request, func(context.Context, idpstore.TxStore, idpadminstore.TxStore, idpadmin.ActionClaims) ([]byte, error) {
		return nil, errors.New("must not run")
	})
	require.ErrorIs(t, err, idpadminapp.ErrSecretAlreadyIssued)
	record, err := store.GetIdempotencyRecord(ctx, principal.Subject, request.IdempotencyKey)
	require.NoError(t, err)
	require.NotContains(t, string(record.Response), "only-once")
}

func executorGrant(now time.Time) idpadmin.Grant {
	return idpadmin.Grant{
		ID: "grant-1", ActorSubject: "owner-sub", Scope: idpadmin.SystemScope(), Role: "owner",
		Capabilities: idpadmin.AllCapabilities(), Version: 1, IssuedAt: now.Add(-time.Hour),
	}
}

func executorPrincipal(now time.Time) idpadmin.AdminPrincipal {
	return idpadmin.AdminPrincipal{
		Subject: "owner-sub", SessionID: "session-1", Authenticated: now,
		Assurance: idpadmin.AssuranceFresh,
	}
}

func newExecutorFixture(t *testing.T, store idpadminstore.Store, now *time.Time) (*idpadmin.HandleService, *idpadmin.Authorizer, *idpadminapp.Executor) {
	t.Helper()
	clock := func() time.Time { return *now }
	handles, err := idpadmin.NewHandleService([]byte("0123456789abcdef0123456789abcdef"), time.Minute, clock)
	require.NoError(t, err)
	authorizer, err := idpadmin.NewAuthorizer(store, 5*time.Minute, clock)
	require.NoError(t, err)
	executor, err := idpadminapp.NewExecutor(store, handles, authorizer, clock)
	require.NoError(t, err)
	return handles, authorizer, executor
}

func seedResourceVersion(
	ctx context.Context,
	store *sqlitestore.Store,
	resourceType, resourceID string,
	version int64,
	now time.Time,
) error {
	_, err := store.SQLDB().ExecContext(ctx, `
		INSERT INTO admin_resource_versions(resource_type, resource_id, version, updated_at_ns)
		VALUES (?, ?, ?, ?)`, resourceType, resourceID, version, now.UnixNano())
	return err
}

func assertTableCount(t *testing.T, store *sqlitestore.Store, table string, expected int) {
	t.Helper()
	var count int
	require.NoError(t, store.SQLDB().QueryRow(`SELECT COUNT(*) FROM `+table).Scan(&count)) //nolint:gosec // closed test table names.
	require.Equal(t, expected, count)
}
