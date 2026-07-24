package sqlitestore_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-go-golems/tiny-idp/pkg/idpadmin"
	"github.com/go-go-golems/tiny-idp/pkg/idpadminstore"
	"github.com/go-go-golems/tiny-idp/pkg/idpstore"
)

func TestAdminGrantLifecycleAndSingleOwner(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	now := time.Now().UTC()
	grant := testAdminGrant(now)
	if err := st.CreateAdminGrant(ctx, grant); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetAdminGrant(ctx, grant.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ActorSubject != grant.ActorSubject || !got.Has(idpadmin.CapabilityUsersWrite) {
		t.Fatalf("grant = %#v", got)
	}
	second := grant
	second.ID = "grant-2"
	if err := st.CreateAdminGrant(ctx, second); !errors.Is(err, idpadminstore.ErrDuplicate) {
		t.Fatalf("second active owner error = %v", err)
	}
	if err := st.RevokeAdminGrant(ctx, grant.ID, grant.Version+1, now); !errors.Is(err, idpadminstore.ErrVersionConflict) {
		t.Fatalf("stale revoke error = %v", err)
	}
	if err := st.RevokeAdminGrant(ctx, grant.ID, grant.Version, now); err != nil {
		t.Fatal(err)
	}
}

func TestAdminUpdateRollsBackProtocolAndControlPlaneTogether(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	want := errors.New("injected failure")
	grant := testAdminGrant(time.Now().UTC())
	err := st.AdminUpdate(ctx, func(protocol idpstore.TxStore, admin idpadminstore.TxStore) error {
		if err := protocol.PutUser(ctx, "alice", idpstore.User{ID: "user-1", Sub: "subject-1"}); err != nil {
			return err
		}
		if err := admin.CreateAdminGrant(ctx, grant); err != nil {
			return err
		}
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("AdminUpdate error = %v", err)
	}
	if _, err := st.GetUser(ctx, "user-1"); !errors.Is(err, idpstore.ErrNotFound) {
		t.Fatalf("rolled-back user error = %v", err)
	}
	if _, err := st.GetAdminGrant(ctx, grant.ID); !errors.Is(err, idpadmin.ErrGrantNotFound) {
		t.Fatalf("rolled-back grant error = %v", err)
	}
}

func TestActionNonceAndResourceVersionAreCompareAndSet(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	now := time.Now().UTC()
	if err := st.CreateActionNonce(ctx, "nonce-1", "session-1", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := st.ConsumeActionNonce(ctx, "nonce-1", "session-1", now); err != nil {
		t.Fatal(err)
	}
	if err := st.ConsumeActionNonce(ctx, "nonce-1", "session-1", now); !errors.Is(err, idpadminstore.ErrNonceConsumed) {
		t.Fatalf("replay error = %v", err)
	}
	if _, err := st.SQLDB().ExecContext(ctx, `
		INSERT INTO admin_resource_versions(resource_type, resource_id, version, updated_at_ns)
		VALUES ('user', 'user-1', 4, ?)`, now.UnixNano()); err != nil {
		t.Fatal(err)
	}
	version, err := st.CompareAndIncrementResourceVersion(ctx, "user", "user-1", 4, now)
	if err != nil || version != 5 {
		t.Fatalf("increment = %d, %v", version, err)
	}
	if _, err := st.CompareAndIncrementResourceVersion(ctx, "user", "user-1", 4, now); !errors.Is(err, idpadminstore.ErrVersionConflict) {
		t.Fatalf("stale CAS error = %v", err)
	}
}

func TestAdminUserProjectionRebuildMatchesCanonicalState(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	now := time.Date(2026, 7, 23, 23, 0, 0, 0, time.UTC)
	created := now.Add(-24 * time.Hour)
	lastLogin := now.Add(-time.Hour)
	requireNoError(t, st.PutUser(ctx, "alice", idpstore.User{
		ID: "user-1", Sub: "subject-1", Email: "alice@example.com", Name: "Alice",
		CreatedAt: created, UpdatedAt: lastLogin,
	}))
	requireNoError(t, st.PutAccountSecurityState(ctx, idpstore.AccountSecurityState{
		UserID: "user-1", LastSuccessfulLoginAt: &lastLogin,
	}))
	requireNoError(t, st.CreateSession(ctx, idpstore.Session{
		IDHash: []byte("session-hash"), UserID: "user-1", ExpiresAt: now.Add(time.Hour),
	}))
	requireNoError(t, st.CreateGrant(ctx, idpstore.Grant{
		ID: "oidc-grant-1", UserID: "user-1", ExpiresAt: now.Add(time.Hour),
	}))

	report, err := st.RebuildAdminUserProjection(ctx, now)
	requireNoError(t, err)
	if report.SourceRows != 1 || report.ProjectionRows != 1 || report.Mismatches != 0 {
		t.Fatalf("rebuild report = %#v", report)
	}
	report, err = st.CheckAdminUserProjection(ctx, now)
	requireNoError(t, err)
	if report.Mismatches != 0 {
		t.Fatalf("check report = %#v", report)
	}
	var sessions, grants, version int
	requireNoError(t, st.SQLDB().QueryRowContext(ctx, `
		SELECT active_session_count, active_grant_count, version
		FROM admin_user_projection WHERE user_id='user-1'`).Scan(&sessions, &grants, &version))
	if sessions != 1 || grants != 1 || version != 1 {
		t.Fatalf("projection counts/version = %d/%d/%d", sessions, grants, version)
	}

	_, err = st.SQLDB().ExecContext(ctx, `
		UPDATE admin_user_projection SET email='drift@example.com' WHERE user_id='user-1'`)
	requireNoError(t, err)
	report, err = st.CheckAdminUserProjection(ctx, now)
	requireNoError(t, err)
	if report.Mismatches != 1 {
		t.Fatalf("drift report = %#v", report)
	}
}

func TestAdminAuthAttemptIsBoundAndConsumedOnce(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	now := time.Date(2026, 7, 24, 1, 0, 0, 0, time.UTC)
	attempt := idpadminstore.AuthAttempt{
		StateHash: []byte("state-hash"), NonceHash: []byte("nonce-hash"),
		PKCEVerifierBox: []byte("encrypted-verifier"), ReturnPath: "/admin/users",
		BrowserBindingHash: []byte("browser-binding"), CreatedAt: now, ExpiresAt: now.Add(5 * time.Minute),
	}
	requireNoError(t, st.CreateAdminAuthAttempt(ctx, attempt))
	_, err := st.ConsumeAdminAuthAttempt(ctx, attempt.StateHash, []byte("wrong-binding"), now)
	if !errors.Is(err, idpadminstore.ErrNotFound) {
		t.Fatalf("wrong binding error = %v", err)
	}
	got, err := st.ConsumeAdminAuthAttempt(ctx, attempt.StateHash, attempt.BrowserBindingHash, now)
	requireNoError(t, err)
	if got.ReturnPath != attempt.ReturnPath || got.ConsumedAt == nil {
		t.Fatalf("consumed attempt = %#v", got)
	}
	_, err = st.ConsumeAdminAuthAttempt(ctx, attempt.StateHash, attempt.BrowserBindingHash, now)
	if !errors.Is(err, idpadminstore.ErrNotFound) {
		t.Fatalf("replay error = %v", err)
	}
}

func requireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func testAdminGrant(now time.Time) idpadmin.Grant {
	return idpadmin.Grant{
		ID: "grant-1", ActorSubject: "subject-1", Scope: idpadmin.SystemScope(), Role: "owner",
		Capabilities: idpadmin.AllCapabilities(), Version: 1, IssuedAt: now,
	}
}
