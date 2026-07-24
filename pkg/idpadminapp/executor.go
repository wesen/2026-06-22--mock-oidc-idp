package idpadminapp

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/go-go-golems/tiny-idp/pkg/idpadmin"
	"github.com/go-go-golems/tiny-idp/pkg/idpadminstore"
	"github.com/go-go-golems/tiny-idp/pkg/idpstore"
	pkgerrors "github.com/pkg/errors"
)

var (
	ErrIdempotencyRequired = errors.New("idempotency key and request hash are required")
	ErrSecretAlreadyIssued = errors.New("one-time secret operation was already executed")
)

type ExecutionPolicy struct {
	SecretBearing bool
}

type ExecutionRequest struct {
	Handle         string
	Principal      idpadmin.AdminPrincipal
	IdempotencyKey string
	RequestHash    []byte
	Policy         ExecutionPolicy
}

type Mutation func(context.Context, idpstore.TxStore, idpadminstore.TxStore, idpadmin.ActionClaims) ([]byte, error)

type Executor struct {
	store      idpadminstore.Store
	handles    *idpadmin.HandleService
	authorizer *idpadmin.Authorizer
	now        func() time.Time
	id         func() (string, error)
}

func NewExecutor(
	store idpadminstore.Store,
	handles *idpadmin.HandleService,
	authorizer *idpadmin.Authorizer,
	now func() time.Time,
) (*Executor, error) {
	if store == nil || handles == nil || authorizer == nil {
		return nil, errors.New("executor store, handle service, and authorizer are required")
	}
	if now == nil {
		now = time.Now
	}
	return &Executor{store: store, handles: handles, authorizer: authorizer, now: now, id: randomID}, nil
}

func (e *Executor) Execute(ctx context.Context, request ExecutionRequest, mutate Mutation) ([]byte, error) {
	if mutate == nil {
		return nil, errors.New("mutation callback is required")
	}
	if strings.TrimSpace(request.IdempotencyKey) == "" || len(request.RequestHash) < 16 {
		return nil, ErrIdempotencyRequired
	}
	claims, err := e.handles.Verify(request.Handle, request.Principal)
	if err != nil {
		return nil, err
	}
	if _, err := e.authorizer.Authorize(
		ctx, request.Principal, claims.GrantID, claims.GrantVersion, claims.Scope,
		claims.Capability, claims.RequireFresh,
	); err != nil {
		return nil, err
	}
	actionID, err := e.id()
	if err != nil {
		return nil, pkgerrors.Wrap(err, "generate admin action id")
	}
	now := e.now().UTC()
	var response []byte
	err = e.store.AdminUpdate(ctx, func(protocol idpstore.TxStore, admin idpadminstore.TxStore) error {
		previous, err := admin.GetIdempotencyRecord(ctx, request.Principal.Subject, request.IdempotencyKey)
		switch {
		case err == nil:
			if subtle.ConstantTimeCompare(previous.RequestHash, request.RequestHash) != 1 {
				return idpadminstore.ErrIdempotencyConflict
			}
			if request.Policy.SecretBearing {
				return ErrSecretAlreadyIssued
			}
			response = append([]byte(nil), previous.Response...)
			return nil
		case !errors.Is(err, idpadminstore.ErrNotFound):
			return err
		}
		if err := admin.ConsumeActionNonce(ctx, claims.Nonce, claims.SessionID, now); err != nil {
			return err
		}
		if claims.ExpectedVersion > 0 {
			if _, err := admin.CompareAndIncrementResourceVersion(
				ctx, claims.TargetType, claims.TargetID, claims.ExpectedVersion, now,
			); err != nil {
				return err
			}
		}
		response, err = mutate(ctx, protocol, admin, claims)
		if err != nil {
			return err
		}
		if err := admin.InsertAdminAction(ctx, idpadminstore.Action{
			ID: actionID, Nonce: claims.Nonce, Subject: claims.Subject, GrantID: claims.GrantID,
			GrantVersion: claims.GrantVersion, Capability: claims.Capability, Command: claims.Command,
			TargetType: claims.TargetType, TargetID: claims.TargetID, ExpectedVersion: claims.ExpectedVersion,
			Status: "succeeded", CreatedAt: now, CompletedAt: &now,
		}); err != nil {
			return pkgerrors.Wrap(err, "insert admin action")
		}
		auditPayload, err := json.Marshal(map[string]any{
			"action_id": actionID, "command": claims.Command, "subject": claims.Subject,
			"target_type": claims.TargetType, "target_id": claims.TargetID, "result": "accepted",
		})
		if err != nil {
			return pkgerrors.Wrap(err, "encode audit outbox payload")
		}
		if err := admin.EnqueueAudit(ctx, idpadminstore.AuditOutboxRecord{
			ID: actionID + "-audit", ActionID: actionID, EventType: claims.Command,
			Payload: auditPayload, CreatedAt: now,
		}); err != nil {
			return pkgerrors.Wrap(err, "enqueue admin audit")
		}
		storedResponse := response
		if request.Policy.SecretBearing {
			storedResponse = []byte(`{"executed":true,"secret_replay":false}`)
		}
		if err := admin.PutIdempotencyRecord(ctx, idpadminstore.IdempotencyRecord{
			Subject: request.Principal.Subject, Key: request.IdempotencyKey,
			RequestHash: append([]byte(nil), request.RequestHash...), StatusCode: 200,
			Response: append([]byte(nil), storedResponse...), CreatedAt: now, ExpiresAt: now.Add(24 * time.Hour),
		}); err != nil {
			return pkgerrors.Wrap(err, "store admin idempotency result")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return response, nil
}
