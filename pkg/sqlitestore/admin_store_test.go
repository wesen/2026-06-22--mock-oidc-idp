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

func testAdminGrant(now time.Time) idpadmin.Grant {
	return idpadmin.Grant{
		ID: "grant-1", ActorSubject: "subject-1", Scope: idpadmin.SystemScope(), Role: "owner",
		Capabilities: idpadmin.AllCapabilities(), Version: 1, IssuedAt: now,
	}
}
