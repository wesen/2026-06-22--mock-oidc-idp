package idpadminapp

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-go-golems/tiny-idp/pkg/idp"
	"github.com/go-go-golems/tiny-idp/pkg/idpadminstore"
)

type outboxStoreStub struct {
	records      []idpadminstore.AuditOutboxRecord
	health       idpadminstore.OutboxHealth
	deliveredIDs []string
	failedIDs    []string
	nextAttempt  time.Time
}

func (s *outboxStoreStub) ListPendingAudit(context.Context, time.Time, int) ([]idpadminstore.AuditOutboxRecord, error) {
	return append([]idpadminstore.AuditOutboxRecord(nil), s.records...), nil
}
func (s *outboxStoreStub) MarkAuditDelivered(_ context.Context, id string, _ time.Time) error {
	s.deliveredIDs = append(s.deliveredIDs, id)
	return nil
}
func (s *outboxStoreStub) RecordAuditFailure(_ context.Context, id, _ string, next time.Time) error {
	s.failedIDs = append(s.failedIDs, id)
	s.nextAttempt = next
	return nil
}
func (s *outboxStoreStub) GetAuditOutboxHealth(context.Context) (idpadminstore.OutboxHealth, error) {
	return s.health, nil
}
func (*outboxStoreStub) CreateAdminOperation(context.Context, idpadminstore.Operation) error {
	return nil
}
func (*outboxStoreStub) ClaimAdminOperation(context.Context, string, time.Time) (bool, error) {
	return false, nil
}
func (*outboxStoreStub) ListPendingAdminOperations(context.Context, int) ([]idpadminstore.Operation, error) {
	return nil, nil
}
func (*outboxStoreStub) CompleteAdminOperation(context.Context, string, []byte, string, time.Time) error {
	return nil
}
func (*outboxStoreStub) FailAdminOperation(context.Context, string, string, time.Time) error {
	return nil
}

type failingSink struct{}

func (failingSink) Emit(context.Context, idp.Event) error { return errors.New("unavailable") }

func TestOutboxWorkerDeliversSafeEventOnce(t *testing.T) {
	now := time.Date(2026, time.July, 23, 14, 0, 0, 0, time.UTC)
	store := &outboxStoreStub{records: []idpadminstore.AuditOutboxRecord{{
		ID: "audit-1", ActionID: "action-1", EventType: "users.create", CreatedAt: now,
		Payload: []byte(`{"request_id":"request-1","command":"users.create","subject":"operator-1","capability":"users.create","target_type":"user","target_id":"user-1","expected_version":0,"resulting_version":1,"operator_reason":"","assurance":"fresh","result":"accepted"}`),
	}}}
	sink := idp.NewMemorySink()
	worker, err := NewOutboxWorker(store, sink, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	delivered, err := worker.DrainOnce(context.Background())
	if err != nil || delivered != 1 || len(store.deliveredIDs) != 1 {
		t.Fatalf("delivery = %d, %#v, %v", delivered, store.deliveredIDs, err)
	}
	events := sink.Events()
	if len(events) != 1 || events[0].Subject != "operator-1" ||
		events[0].Fields["target_id"] != "user-1" {
		t.Fatalf("events = %#v", events)
	}
}

func TestOutboxWorkerPersistsBoundedRetryAndReadiness(t *testing.T) {
	now := time.Date(2026, time.July, 23, 14, 0, 0, 0, time.UTC)
	oldest := now.Add(-maximumAuditAge)
	store := &outboxStoreStub{
		records: []idpadminstore.AuditOutboxRecord{{
			ID: "audit-1", Attempts: 20, CreatedAt: oldest,
			Payload: []byte(`{"command":"users.create","subject":"operator-1"}`),
		}},
		health: idpadminstore.OutboxHealth{
			Pending: 1, OldestPending: &oldest, LastErrorCode: auditDeliveryErrorCode,
		},
	}
	worker, err := NewOutboxWorker(store, failingSink{}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	delivered, err := worker.DrainOnce(context.Background())
	if err != nil || delivered != 0 || len(store.failedIDs) != 1 {
		t.Fatalf("failure = %d, %#v, %v", delivered, store.failedIDs, err)
	}
	if got := store.nextAttempt.Sub(now); got != maximumAuditBackoff {
		t.Fatalf("backoff = %s, want %s", got, maximumAuditBackoff)
	}
	check := worker.Readiness(context.Background())
	if check.Ready || !check.Degraded || check.Reason != auditDeliveryErrorCode {
		t.Fatalf("readiness = %#v", check)
	}
}

func TestOutboxWorkerStopsOnCancellation(t *testing.T) {
	store := &outboxStoreStub{}
	worker, err := NewOutboxWorker(store, idp.NewMemorySink(), time.Now)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := worker.Run(ctx); err != nil {
		t.Fatal(err)
	}
}

var _ idpadminstore.WorkerStore = (*outboxStoreStub)(nil)
