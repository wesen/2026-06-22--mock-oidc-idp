// Package idpadminstore defines persistence contracts for the administration
// control plane. It depends on domain values, never on HTTP or Widget DSL.
package idpadminstore

import (
	"context"
	"errors"
	"time"

	"github.com/go-go-golems/tiny-idp/pkg/idpadmin"
	"github.com/go-go-golems/tiny-idp/pkg/idpstore"
)

var (
	ErrNotFound            = errors.New("administration record not found")
	ErrDuplicate           = errors.New("administration record already exists")
	ErrNonceConsumed       = errors.New("administration action nonce already consumed")
	ErrVersionConflict     = errors.New("administration resource version conflict")
	ErrIdempotencyConflict = errors.New("idempotency key was used for another request")
)

type Session struct {
	IDHash          []byte
	Subject         string
	GrantID         string
	GrantVersion    int64
	CSRFHash        []byte
	AuthenticatedAt time.Time
	CreatedAt       time.Time
	LastSeenAt      time.Time
	ExpiresAt       time.Time
	RevokedAt       *time.Time
}

type AuthAttempt struct {
	StateHash          []byte
	NonceHash          []byte
	PKCEVerifierBox    []byte
	ReturnPath         string
	BrowserBindingHash []byte
	CreatedAt          time.Time
	ExpiresAt          time.Time
	ConsumedAt         *time.Time
}

type Action struct {
	ID               string
	RequestID        string
	SessionBinding   string
	Nonce            string
	Subject          string
	GrantID          string
	GrantVersion     int64
	Scope            idpadmin.AdminScope
	Capability       idpadmin.Capability
	Command          string
	TargetType       string
	TargetID         string
	ExpectedVersion  int64
	ResultingVersion int64
	Reason           string
	Assurance        idpadmin.Assurance
	Status           string
	ErrorCode        string
	CreatedAt        time.Time
	CompletedAt      *time.Time
}

type IdempotencyRecord struct {
	Subject     string
	Key         string
	RequestHash []byte
	StatusCode  int
	Response    []byte
	CreatedAt   time.Time
	ExpiresAt   time.Time
}

type AuditOutboxRecord struct {
	ID            string
	ActionID      string
	EventType     string
	Payload       []byte
	CreatedAt     time.Time
	DeliveredAt   *time.Time
	Attempts      int
	LastError     string
	NextAttemptAt time.Time
}

type Operation struct {
	ID                 string
	Kind               string
	Command            string
	ActorSubject       string
	Status             string
	Label              string
	Progress           []byte
	Result             []byte
	RelativeResultPath string
	ErrorCode          string
	CreatedAt          time.Time
	UpdatedAt          time.Time
	StartedAt          *time.Time
	CompletedAt        *time.Time
}

type OutboxHealth struct {
	Pending       int64
	OldestPending *time.Time
	LastErrorCode string
}

type ProjectionReport struct {
	SourceRows     int `json:"source_rows"`
	ProjectionRows int `json:"projection_rows"`
	Mismatches     int `json:"mismatches"`
}

type InvitationRecord struct {
	InvitationID     string
	Label            string
	CreatedBySubject string
	CreatedAt        time.Time
	LastIssuedAt     time.Time
}

type GrantStore interface {
	CreateAdminGrant(ctx context.Context, grant idpadmin.Grant) error
	GetAdminGrant(ctx context.Context, id string) (idpadmin.Grant, error)
	GetActiveSystemOwner(ctx context.Context, now time.Time) (idpadmin.Grant, error)
	FindActiveAdminGrant(ctx context.Context, subject string, scope idpadmin.AdminScope, now time.Time) (idpadmin.Grant, error)
	RevokeAdminGrant(ctx context.Context, id string, expectedVersion int64, at time.Time) error
}

type SessionStore interface {
	CreateAdminSession(ctx context.Context, session Session) error
	GetAdminSession(ctx context.Context, idHash []byte) (Session, error)
	TouchAdminSession(ctx context.Context, idHash []byte, seenAt time.Time) error
	RotateAdminSessionCSRF(ctx context.Context, idHash, csrfHash []byte, now time.Time) error
	RevokeAdminSession(ctx context.Context, idHash []byte, at time.Time) error
}

type AuthAttemptStore interface {
	CreateAdminAuthAttempt(ctx context.Context, attempt AuthAttempt) error
	ConsumeAdminAuthAttempt(ctx context.Context, stateHash, browserBindingHash []byte, now time.Time) (AuthAttempt, error)
}

type SecurityStore interface {
	CreateActionNonce(ctx context.Context, nonce, sessionID string, expiresAt time.Time) error
	ConsumeActionNonce(ctx context.Context, nonce, sessionID string, now time.Time) error
	CreateResourceVersion(ctx context.Context, resourceType, resourceID string, version int64, now time.Time) error
	GetResourceVersion(ctx context.Context, resourceType, resourceID string) (int64, error)
	CompareAndIncrementResourceVersion(ctx context.Context, resourceType, resourceID string, expected int64, now time.Time) (int64, error)
	PutIdempotencyRecord(ctx context.Context, record IdempotencyRecord) error
	GetIdempotencyRecord(ctx context.Context, subject, key string) (IdempotencyRecord, error)
	InsertAdminAction(ctx context.Context, action Action) error
	EnqueueAudit(ctx context.Context, record AuditOutboxRecord) error
}

type WorkerStore interface {
	ListPendingAudit(ctx context.Context, now time.Time, limit int) ([]AuditOutboxRecord, error)
	MarkAuditDelivered(ctx context.Context, id string, deliveredAt time.Time) error
	RecordAuditFailure(ctx context.Context, id, errorCode string, nextAttemptAt time.Time) error
	GetAuditOutboxHealth(ctx context.Context) (OutboxHealth, error)
	CreateAdminOperation(ctx context.Context, operation Operation) error
	ClaimAdminOperation(ctx context.Context, id string, startedAt time.Time) (bool, error)
	ListPendingAdminOperations(ctx context.Context, limit int) ([]Operation, error)
	CompleteAdminOperation(ctx context.Context, id string, result []byte, relativePath string, completedAt time.Time) error
	FailAdminOperation(ctx context.Context, id, errorCode string, completedAt time.Time) error
}

type InvitationStore interface {
	CreateAdminInvitation(ctx context.Context, record InvitationRecord) error
}

type TxStore interface {
	GrantStore
	SessionStore
	AuthAttemptStore
	SecurityStore
	InvitationStore
}

type AtomicStore interface {
	AdminUpdate(ctx context.Context, fn func(idpstore.TxStore, TxStore) error) error
}

type ProjectionStore interface {
	RebuildAdminUserProjection(ctx context.Context, now time.Time) (ProjectionReport, error)
	CheckAdminUserProjection(ctx context.Context, now time.Time) (ProjectionReport, error)
	InitializeAdminResourceVersions(ctx context.Context, now time.Time) error
}

type ReadModelStore interface {
	GetAdminOverview(ctx context.Context, now time.Time) (idpadmin.Overview, error)
	GetAdminUser(ctx context.Context, userID string) (idpadmin.UserRow, error)
	ListAdminUsers(ctx context.Context, filter idpadmin.UserFilter, limit int) ([]idpadmin.UserRow, error)
	ListAdminActivity(ctx context.Context, limit int) ([]idpadmin.ActivityRow, error)
	ListAdminOperations(ctx context.Context, limit int) (idpadmin.OperationsView, error)
	ListAdminInvitations(ctx context.Context, now time.Time, limit int) ([]idpadmin.InvitationRow, error)
}

// UserProjectionTx refreshes one derived user row inside the same transaction
// as its protocol mutation.
type UserProjectionTx interface {
	RefreshAdminUserProjection(ctx context.Context, userID string, now time.Time) error
}

type Store interface {
	TxStore
	AtomicStore
	ProjectionStore
	ReadModelStore
	WorkerStore
}
