package idpadminapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/go-go-golems/tiny-idp/pkg/idp"
	"github.com/go-go-golems/tiny-idp/pkg/idpadminstore"
	pkgerrors "github.com/pkg/errors"
)

const (
	auditDeliveryErrorCode = "audit_delivery_failed"
	defaultOutboxBatchSize = 25
	defaultOutboxPoll      = time.Second
	maximumAuditAge        = 5 * time.Minute
	maximumAuditBackoff    = time.Minute
)

// OutboxWorker delivers committed administration audit events without making
// the browser request depend on the external sink.
type OutboxWorker struct {
	store        idpadminstore.WorkerStore
	sink         idp.Sink
	now          func() time.Time
	pollInterval time.Duration
}

func NewOutboxWorker(
	store idpadminstore.WorkerStore,
	sink idp.Sink,
	now func() time.Time,
) (*OutboxWorker, error) {
	if store == nil || sink == nil {
		return nil, errors.New("outbox worker store and audit sink are required")
	}
	if now == nil {
		now = time.Now
	}
	return &OutboxWorker{
		store: store, sink: sink, now: now, pollInterval: defaultOutboxPoll,
	}, nil
}

// Run drains immediately, then polls until cancellation. Cancellation is a
// normal shutdown condition and is therefore returned as nil.
func (w *OutboxWorker) Run(ctx context.Context) error {
	// Per-record delivery failures are persisted and retried. A store failure
	// is transient too, so keep the production worker alive.
	_, _ = w.DrainOnce(ctx)
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			_, _ = w.DrainOnce(ctx)
		}
	}
}

// DrainOnce processes one bounded batch and returns the number delivered.
func (w *OutboxWorker) DrainOnce(ctx context.Context) (int, error) {
	now := w.now().UTC()
	records, err := w.store.ListPendingAudit(ctx, now, defaultOutboxBatchSize)
	if err != nil {
		return 0, pkgerrors.Wrap(err, "list pending administration audit")
	}
	delivered := 0
	for _, record := range records {
		event, err := auditEvent(record)
		if err == nil {
			err = w.sink.Emit(ctx, event)
		}
		if err != nil {
			next := now.Add(auditBackoff(record.Attempts + 1))
			if storeErr := w.store.RecordAuditFailure(
				ctx, record.ID, auditDeliveryErrorCode, next,
			); storeErr != nil {
				return delivered, pkgerrors.Wrap(storeErr, "record administration audit failure")
			}
			continue
		}
		if err := w.store.MarkAuditDelivered(ctx, record.ID, now); err != nil {
			return delivered, pkgerrors.Wrap(err, "mark administration audit delivered")
		}
		delivered++
	}
	return delivered, nil
}

func (w *OutboxWorker) Readiness(ctx context.Context) idp.ReadinessCheck {
	now := w.now().UTC()
	check := idp.ReadinessCheck{Name: "admin_audit_outbox", Ready: true, CheckedAt: now}
	health, err := w.store.GetAuditOutboxHealth(ctx)
	if err != nil {
		check.Ready = false
		check.Reason = "audit_outbox_health_failed"
		return check
	}
	if health.Pending == 0 {
		return check
	}
	check.Degraded = true
	check.Reason = "audit_delivery_pending"
	if health.LastErrorCode != "" {
		check.Reason = health.LastErrorCode
	}
	if health.OldestPending != nil && now.Sub(*health.OldestPending) >= maximumAuditAge {
		check.Ready = false
	}
	return check
}

func auditBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	backoff := time.Second
	for i := 1; i < attempt && backoff < maximumAuditBackoff; i++ {
		backoff *= 2
	}
	if backoff > maximumAuditBackoff {
		return maximumAuditBackoff
	}
	return backoff
}

func auditEvent(record idpadminstore.AuditOutboxRecord) (idp.Event, error) {
	var payload struct {
		RequestID        string `json:"request_id"`
		Command          string `json:"command"`
		Subject          string `json:"subject"`
		Scope            any    `json:"scope"`
		Capability       string `json:"capability"`
		TargetType       string `json:"target_type"`
		TargetID         string `json:"target_id"`
		ExpectedVersion  int64  `json:"expected_version"`
		ResultingVersion int64  `json:"resulting_version"`
		OperatorReason   string `json:"operator_reason"`
		Assurance        string `json:"assurance"`
		Result           string `json:"result"`
	}
	if err := json.Unmarshal(record.Payload, &payload); err != nil {
		return idp.Event{}, pkgerrors.Wrap(err, "decode administration audit payload")
	}
	if payload.Command == "" || payload.Subject == "" {
		return idp.Event{}, fmt.Errorf("administration audit payload is incomplete")
	}
	return idp.Event{
		Time: record.CreatedAt, Name: payload.Command, Subject: payload.Subject,
		RequestID: payload.RequestID, Result: payload.Result, Reason: payload.OperatorReason,
		Fields: map[string]string{
			"action_id":         record.ActionID,
			"capability":        payload.Capability,
			"target_type":       payload.TargetType,
			"target_id":         payload.TargetID,
			"expected_version":  strconv.FormatInt(payload.ExpectedVersion, 10),
			"resulting_version": strconv.FormatInt(payload.ResultingVersion, 10),
			"assurance":         payload.Assurance,
		},
	}, nil
}
