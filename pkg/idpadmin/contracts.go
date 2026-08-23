package idpadmin

import (
	"context"
	"time"
)

const (
	DefaultPageSize = 25
	MaxPageSize     = 100
	MaxSearchLength = 200
)

type CursorPageRequest struct {
	Cursor string
	Limit  int
	Sort   string
}

func (p CursorPageRequest) Normalized() CursorPageRequest {
	if p.Limit <= 0 {
		p.Limit = DefaultPageSize
	}
	if p.Limit > MaxPageSize {
		p.Limit = MaxPageSize
	}
	return p
}

type CursorPage[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
}

type Overview struct {
	UserCount              int64      `json:"user_count"`
	DisabledUserCount      int64      `json:"disabled_user_count"`
	ActiveClientCount      int64      `json:"active_client_count"`
	PendingInvitationCount int64      `json:"pending_invitation_count"`
	PendingOperationCount  int64      `json:"pending_operation_count"`
	SchemaVersion          int        `json:"schema_version"`
	LastBackupAt           *time.Time `json:"last_backup_at,omitempty"`
}

type UserStatus string

const (
	UserStatusAll      UserStatus = ""
	UserStatusActive   UserStatus = "active"
	UserStatusDisabled UserStatus = "disabled"
	UserStatusLocked   UserStatus = "locked"
)

type UserFilter struct {
	Query  string
	Status UserStatus
}

type UserRow struct {
	ID                    string     `json:"id"`
	Subject               string     `json:"subject"`
	Login                 string     `json:"login"`
	Email                 string     `json:"email"`
	DisplayName           string     `json:"display_name"`
	Disabled              bool       `json:"disabled"`
	LockedUntil           *time.Time `json:"locked_until,omitempty"`
	LastSuccessfulLoginAt *time.Time `json:"last_successful_login_at,omitempty"`
	ActiveSessionCount    int        `json:"active_session_count"`
	ActiveGrantCount      int        `json:"active_grant_count"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
	Version               int64      `json:"version"`
}

type UserDetail struct {
	UserRow
	EmailVerified      bool       `json:"email_verified"`
	PreferredUsername  string     `json:"preferred_username"`
	Groups             []string   `json:"groups"`
	Roles              []string   `json:"roles"`
	Locale             string     `json:"locale"`
	PasswordConfigured bool       `json:"password_configured"`
	PasswordChangedAt  *time.Time `json:"password_changed_at,omitempty"`
}

type InvitationFilter struct {
	Query  string
	Status string
}

type InvitationRow struct {
	ID         string     `json:"id"`
	Label      string     `json:"label"`
	Audience   string     `json:"audience"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	RedeemedAt *time.Time `json:"redeemed_at,omitempty"`
	Version    int64      `json:"version"`
}

type ClientFilter struct {
	Query    string
	Disabled *bool
}

type ClientRow struct {
	ID               string    `json:"id"`
	Public           bool      `json:"public"`
	Disabled         bool      `json:"disabled"`
	RequirePKCE      bool      `json:"require_pkce"`
	SecretConfigured bool      `json:"secret_configured"`
	UpdatedAt        time.Time `json:"updated_at"`
	Version          int64     `json:"version"`
}

type ClientDetail struct {
	ClientRow
	RedirectURIs           []string   `json:"redirect_uris"`
	PostLogoutRedirectURIs []string   `json:"post_logout_redirect_uris"`
	AllowedScopes          []string   `json:"allowed_scopes"`
	AllowedGrantTypes      []string   `json:"allowed_grant_types"`
	AllowedAudiences       []string   `json:"allowed_audiences"`
	CanIntrospect          bool       `json:"can_introspect"`
	LastSecretRotation     *time.Time `json:"last_secret_rotation,omitempty"`
}

type SigningKeyRow struct {
	ID        string     `json:"id"`
	Algorithm string     `json:"algorithm"`
	Active    bool       `json:"active"`
	CreatedAt time.Time  `json:"created_at"`
	NotAfter  *time.Time `json:"not_after,omitempty"`
}

type ActivityFilter struct {
	Query      string
	EventTypes []string
	Result     string
}

type ActivityRow struct {
	ID         string    `json:"id"`
	Subject    string    `json:"subject"`
	EventType  string    `json:"event_type"`
	Command    string    `json:"command"`
	TargetType string    `json:"target_type"`
	TargetID   string    `json:"target_id"`
	Result     string    `json:"result"`
	ErrorCode  string    `json:"error_code,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type OperationRow struct {
	ID          string     `json:"id"`
	Kind        string     `json:"kind"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	ErrorCode   string     `json:"error_code,omitempty"`
}

type OperationsView struct {
	Operations     []OperationRow `json:"operations"`
	PendingOutbox  int64          `json:"pending_outbox"`
	LastAuditError string         `json:"last_audit_error,omitempty"`
}

type QueryService interface {
	GetOverview(context.Context, AdminPrincipal, AdminScope) (Overview, error)
	ListUsers(context.Context, AdminPrincipal, AdminScope, UserFilter, CursorPageRequest) (CursorPage[UserRow], error)
	GetUser(context.Context, AdminPrincipal, AdminScope, string) (UserDetail, error)
	ListInvitations(context.Context, AdminPrincipal, AdminScope, InvitationFilter, CursorPageRequest) (CursorPage[InvitationRow], error)
	ListClients(context.Context, AdminPrincipal, AdminScope, ClientFilter, CursorPageRequest) (CursorPage[ClientRow], error)
	GetClient(context.Context, AdminPrincipal, AdminScope, string) (ClientDetail, error)
	ListSigningKeys(context.Context, AdminPrincipal, AdminScope) ([]SigningKeyRow, error)
	ListActivity(context.Context, AdminPrincipal, AdminScope, ActivityFilter, CursorPageRequest) (CursorPage[ActivityRow], error)
	GetOperations(context.Context, AdminPrincipal, AdminScope) (OperationsView, error)
}

type CommandContext struct {
	Principal       AdminPrincipal
	Scope           AdminScope
	RequestID       string
	IdempotencyKey  string
	ExpectedVersion int64
	Reason          string
}

type CreateUserRequest struct {
	Login         string
	Password      []byte
	Email         string
	EmailVerified bool
	DisplayName   string
}

type UpdateUserRequest struct {
	UserID        string
	Email         string
	EmailVerified bool
	DisplayName   string
	Locale        string
}

type SetPasswordRequest struct {
	UserID   string
	Password []byte
}

type IssueInvitationRequest struct {
	Label    string
	Audience string
	ValidFor time.Duration
}

type CreateClientRequest struct {
	ID                     string
	Public                 bool
	RedirectURIs           []string
	PostLogoutRedirectURIs []string
	AllowedScopes          []string
	AllowedGrantTypes      []string
	AllowedAudiences       []string
	RequirePKCE            bool
	CanIntrospect          bool
}

type UpdateClientRequest = CreateClientRequest

type RotateKeyRequest struct {
	Algorithm string
	Overlap   time.Duration
}

type CreateBackupRequest struct {
	Label string
}

type UserResult struct {
	User        UserDetail `json:"user"`
	Committed   bool       `json:"committed"`
	AuditStatus string     `json:"audit_status"`
}

type InvitationResult struct {
	Invitation InvitationRow `json:"invitation"`
}

type ClientResult struct {
	Client ClientDetail `json:"client"`
}

type KeyResult struct {
	Key SigningKeyRow `json:"key"`
}

type OperationResult struct {
	Operation OperationRow `json:"operation"`
}

// OneTimeSecretResult must never be persisted in idempotency records or cached
// by a client after its initial rendering.
type OneTimeSecretResult struct {
	ResourceID string `json:"resource_id"`
	Secret     string `json:"secret"`
}

type CommandService interface {
	CreateUser(context.Context, CommandContext, CreateUserRequest) (UserResult, error)
	UpdateUser(context.Context, CommandContext, UpdateUserRequest) (UserResult, error)
	SetUserDisabled(context.Context, CommandContext, string, bool) (UserResult, error)
	SetUserPassword(context.Context, CommandContext, SetPasswordRequest) (UserResult, error)
	UnlockUser(context.Context, CommandContext, string) (UserResult, error)
	RevokeUserAccess(context.Context, CommandContext, string) (UserResult, error)
	IssueInvitation(context.Context, CommandContext, IssueInvitationRequest) (OneTimeSecretResult, error)
	RevokeInvitation(context.Context, CommandContext, string) (InvitationResult, error)
	CreateClient(context.Context, CommandContext, CreateClientRequest) (ClientResult, error)
	UpdateClient(context.Context, CommandContext, UpdateClientRequest) (ClientResult, error)
	SetClientDisabled(context.Context, CommandContext, string, bool) (ClientResult, error)
	RotateClientSecret(context.Context, CommandContext, string) (OneTimeSecretResult, error)
	RotateSigningKey(context.Context, CommandContext, RotateKeyRequest) (KeyResult, error)
	RetireSigningKey(context.Context, CommandContext, string) (KeyResult, error)
	RunDoctor(context.Context, CommandContext) (OperationResult, error)
	CreateManagedBackup(context.Context, CommandContext, CreateBackupRequest) (OperationResult, error)
	VerifyManagedBackup(context.Context, CommandContext, string) (OperationResult, error)
}
