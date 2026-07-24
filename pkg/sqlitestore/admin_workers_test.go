package sqlitestore_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-go-golems/tiny-idp/pkg/idpadmin"
	"github.com/go-go-golems/tiny-idp/pkg/idpadminstore"
	"github.com/go-go-golems/tiny-idp/pkg/sqlitestore"
)

func TestAuditOutboxRetryAndDeliveryLifecycle(t *testing.T) {
	ctx := context.Background()
	store, err := sqlitestore.Open(ctx, sqlitestore.DefaultConfig(filepath.Join(t.TempDir(), "idp.db")))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	createdAt := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	record := idpadminstore.AuditOutboxRecord{
		ID: "audit-1", ActionID: "action-1", EventType: "users.create",
		Payload: []byte(`{"result":"accepted"}`), CreatedAt: createdAt,
	}
	if err := store.CreateAdminGrant(ctx, idpadmin.Grant{
		ID: "grant-1", ActorSubject: "operator-1", Scope: idpadmin.SystemScope(),
		Role: "owner", Capabilities: idpadmin.AllCapabilities(), Version: 1, IssuedAt: createdAt,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.InsertAdminAction(ctx, idpadminstore.Action{
		ID: record.ActionID, Nonce: "nonce-1", Subject: "operator-1", GrantID: "grant-1",
		GrantVersion: 1, Scope: idpadmin.SystemScope(), Capability: idpadmin.CapabilityUsersCreate,
		Command: record.EventType, Status: "succeeded", CreatedAt: createdAt,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.EnqueueAudit(ctx, record); err != nil {
		t.Fatal(err)
	}
	pending, err := store.ListPendingAudit(ctx, createdAt, 10)
	if err != nil || len(pending) != 1 {
		t.Fatalf("initial pending = %#v, %v", pending, err)
	}

	retryAt := createdAt.Add(time.Minute)
	if err := store.RecordAuditFailure(ctx, record.ID, "audit_delivery_failed", retryAt); err != nil {
		t.Fatal(err)
	}
	pending, err = store.ListPendingAudit(ctx, retryAt.Add(-time.Nanosecond), 10)
	if err != nil || len(pending) != 0 {
		t.Fatalf("pending before retry = %#v, %v", pending, err)
	}
	pending, err = store.ListPendingAudit(ctx, retryAt, 10)
	if err != nil || len(pending) != 1 || pending[0].Attempts != 1 ||
		pending[0].LastError != "audit_delivery_failed" {
		t.Fatalf("pending at retry = %#v, %v", pending, err)
	}
	health, err := store.GetAuditOutboxHealth(ctx)
	if err != nil || health.Pending != 1 || health.OldestPending == nil ||
		!health.OldestPending.Equal(createdAt) || health.LastErrorCode != "audit_delivery_failed" {
		t.Fatalf("health = %#v, %v", health, err)
	}

	deliveredAt := retryAt.Add(time.Second)
	if err := store.MarkAuditDelivered(ctx, record.ID, deliveredAt); err != nil {
		t.Fatal(err)
	}
	pending, err = store.ListPendingAudit(ctx, deliveredAt.Add(time.Hour), 10)
	if err != nil || len(pending) != 0 {
		t.Fatalf("pending after delivery = %#v, %v", pending, err)
	}
	if err := store.MarkAuditDelivered(ctx, record.ID, deliveredAt); !errors.Is(err, idpadminstore.ErrNotFound) {
		t.Fatalf("second delivery error = %v", err)
	}
}

func TestAdminOperationClaimAndTerminalLifecycle(t *testing.T) {
	ctx := context.Background()
	store, err := sqlitestore.Open(ctx, sqlitestore.DefaultConfig(filepath.Join(t.TempDir(), "idp.db")))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	now := time.Date(2026, time.July, 23, 13, 0, 0, 0, time.UTC)
	operation := idpadminstore.Operation{
		ID: "operation-1", Kind: "backup", Command: "operations.backup.create",
		ActorSubject: "operator-1", Status: "pending", Label: "before rotation",
		CreatedAt: now, UpdatedAt: now,
	}
	if err := store.CreateAdminOperation(ctx, operation); err != nil {
		t.Fatal(err)
	}
	pending, err := store.ListPendingAdminOperations(ctx, 10)
	if err != nil || len(pending) != 1 || pending[0].ActorSubject != "operator-1" {
		t.Fatalf("pending = %#v, %v", pending, err)
	}
	claimed, err := store.ClaimAdminOperation(ctx, operation.ID, now.Add(time.Second))
	if err != nil || !claimed {
		t.Fatalf("claim = %v, %v", claimed, err)
	}
	claimed, err = store.ClaimAdminOperation(ctx, operation.ID, now.Add(2*time.Second))
	if err != nil || claimed {
		t.Fatalf("second claim = %v, %v", claimed, err)
	}
	if err := store.CompleteAdminOperation(
		ctx, operation.ID, []byte(`{"verified":true}`), "backups/operation-1.db",
		now.Add(3*time.Second),
	); err != nil {
		t.Fatal(err)
	}
	if err := store.FailAdminOperation(
		ctx, operation.ID, "backup_failed", now.Add(4*time.Second),
	); !errors.Is(err, idpadminstore.ErrNotFound) {
		t.Fatalf("terminal rewrite error = %v", err)
	}
	view, err := store.ListAdminOperations(ctx, 10)
	if err != nil || len(view.Operations) != 1 ||
		view.Operations[0].Status != "completed" || view.Operations[0].CompletedAt == nil {
		t.Fatalf("operations view = %#v, %v", view, err)
	}
}
