package idpadminapp

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/go-go-golems/tiny-idp/pkg/idpadmin"
	"github.com/go-go-golems/tiny-idp/pkg/idpadminstore"
	"github.com/go-go-golems/tiny-idp/pkg/idpstore"
)

const (
	CommandUsersCreate         = "users.create"
	CommandUsersUpdate         = "users.update"
	CommandUsersEnable         = "users.enable"
	CommandUsersDisable        = "users.disable"
	CommandUsersUnlock         = "users.unlock"
	CommandUsersSetPassword    = "users.set_password"
	CommandUsersRevokeAccess   = "users.revoke_access"
	CommandInvitationsIssue    = "invitations.issue"
	CommandInvitationsRevoke   = "invitations.revoke"
	CommandClientsCreate       = "clients.create"
	CommandClientsUpdate       = "clients.update"
	CommandClientsEnable       = "clients.enable"
	CommandClientsDisable      = "clients.disable"
	CommandClientsRotateSecret = "clients.rotate_secret"
)

var (
	ErrUnknownCommand     = errors.New("unknown administration command")
	ErrActionTarget       = errors.New("invalid administration action target")
	ErrReasonRequired     = errors.New("operator reason is required")
	ErrConfirmationFailed = errors.New("typed confirmation did not match")
)

type ActionDefinition struct {
	Command          string
	Capability       idpadmin.Capability
	TargetType       string
	RequireTarget    bool
	RequireFresh     bool
	RequireReason    bool
	ConfirmationText string
	GenerateTarget   bool
	RequireExisting  bool
}

type PrepareActionRequest struct {
	Command  string `json:"command"`
	TargetID string `json:"target_id,omitempty"`
}

type PreparedAction struct {
	Handle           string    `json:"action_handle"`
	Command          string    `json:"command"`
	TargetID         string    `json:"target_id,omitempty"`
	ExpectedVersion  int64     `json:"expected_version,omitempty"`
	RequireFresh     bool      `json:"require_fresh"`
	RequireReason    bool      `json:"require_reason"`
	ConfirmationText string    `json:"confirmation_text,omitempty"`
	ExpiresAt        time.Time `json:"expires_at"`
}

type ActionService struct {
	store      ActionStore
	handles    *idpadmin.HandleService
	authorizer *idpadmin.Authorizer
	now        func() time.Time
}

type ActionStore interface {
	idpadminstore.Store
	idpstore.Store
}

func NewActionService(
	store ActionStore,
	handles *idpadmin.HandleService,
	authorizer *idpadmin.Authorizer,
	now func() time.Time,
) (*ActionService, error) {
	if store == nil || handles == nil || authorizer == nil {
		return nil, errors.New("action store, handle service, and authorizer are required")
	}
	if now == nil {
		now = time.Now
	}
	return &ActionService{store: store, handles: handles, authorizer: authorizer, now: now}, nil
}

func (s *ActionService) Prepare(
	ctx context.Context,
	principal idpadmin.AdminPrincipal,
	request PrepareActionRequest,
) (PreparedAction, error) {
	definition, ok := actionDefinition(request.Command)
	if !ok {
		return PreparedAction{}, ErrUnknownCommand
	}
	targetID := strings.TrimSpace(request.TargetID)
	if definition.RequireTarget && targetID == "" || !definition.RequireTarget && targetID != "" {
		return PreparedAction{}, ErrActionTarget
	}
	if _, err := s.authorizer.Authorize(
		ctx, principal, principal.GrantID, principal.GrantVersion,
		idpadmin.SystemScope(), definition.Capability, definition.RequireFresh,
	); err != nil {
		return PreparedAction{}, err
	}
	var expectedVersion int64
	var err error
	if definition.RequireExisting {
		expectedVersion, err = s.store.GetResourceVersion(ctx, definition.TargetType, targetID)
		if err != nil {
			return PreparedAction{}, err
		}
	} else if definition.GenerateTarget {
		targetID, err = randomID()
		if err != nil {
			return PreparedAction{}, err
		}
	} else if definition.Command == CommandClientsCreate {
		if _, err := s.store.GetClient(ctx, targetID); err == nil {
			return PreparedAction{}, idpstore.ErrDuplicate
		} else if !errors.Is(err, idpstore.ErrNotFound) {
			return PreparedAction{}, err
		}
	}
	raw, err := s.handles.Mint(idpadmin.ActionClaims{
		SessionID: principal.SessionID, Subject: principal.Subject,
		GrantID: principal.GrantID, GrantVersion: principal.GrantVersion,
		Scope: idpadmin.SystemScope(), Capability: definition.Capability,
		Command: definition.Command, TargetType: definition.TargetType,
		TargetID: targetID, ExpectedVersion: expectedVersion,
		RequireFresh: definition.RequireFresh,
	})
	if err != nil {
		return PreparedAction{}, err
	}
	claims, err := s.handles.Verify(raw, principal)
	if err != nil {
		return PreparedAction{}, err
	}
	if err := s.store.CreateActionNonce(ctx, claims.Nonce, claims.SessionID, claims.ExpiresAt); err != nil {
		return PreparedAction{}, err
	}
	return PreparedAction{
		Handle: raw, Command: definition.Command, TargetID: targetID,
		ExpectedVersion: expectedVersion, RequireFresh: definition.RequireFresh,
		RequireReason:    definition.RequireReason,
		ConfirmationText: definition.ConfirmationText, ExpiresAt: claims.ExpiresAt,
	}, nil
}

func actionDefinition(command string) (ActionDefinition, bool) {
	definitions := map[string]ActionDefinition{
		CommandUsersCreate: {
			Command: CommandUsersCreate, Capability: idpadmin.CapabilityUsersCreate,
			TargetType: "user", GenerateTarget: true,
		},
		CommandUsersUpdate: {
			Command: CommandUsersUpdate, Capability: idpadmin.CapabilityUsersUpdate,
			TargetType: "user", RequireTarget: true, RequireExisting: true,
		},
		CommandUsersEnable: {
			Command: CommandUsersEnable, Capability: idpadmin.CapabilityUsersDisable,
			TargetType: "user", RequireTarget: true, RequireExisting: true, RequireReason: true,
		},
		CommandUsersDisable: {
			Command: CommandUsersDisable, Capability: idpadmin.CapabilityUsersDisable,
			TargetType: "user", RequireTarget: true, RequireExisting: true,
			RequireReason: true, ConfirmationText: "DISABLE",
		},
		CommandUsersUnlock: {
			Command: CommandUsersUnlock, Capability: idpadmin.CapabilityUsersUnlock,
			TargetType: "user", RequireTarget: true, RequireExisting: true, RequireReason: true,
		},
		CommandUsersSetPassword: {
			Command: CommandUsersSetPassword, Capability: idpadmin.CapabilityUsersPasswordSet,
			TargetType: "user", RequireTarget: true, RequireExisting: true, RequireFresh: true, RequireReason: true,
		},
		CommandUsersRevokeAccess: {
			Command: CommandUsersRevokeAccess, Capability: idpadmin.CapabilityUsersAccessRevoke,
			TargetType: "user", RequireTarget: true, RequireExisting: true, RequireFresh: true,
			RequireReason: true, ConfirmationText: "REVOKE",
		},
		CommandInvitationsIssue: {
			Command: CommandInvitationsIssue, Capability: idpadmin.CapabilityInvitationsCreate,
			TargetType: "invitation", GenerateTarget: true,
		},
		CommandInvitationsRevoke: {
			Command: CommandInvitationsRevoke, Capability: idpadmin.CapabilityInvitationsRevoke,
			TargetType: "invitation", RequireTarget: true, RequireExisting: true,
			RequireReason: true, ConfirmationText: "REVOKE",
		},
		CommandClientsCreate: {
			Command: CommandClientsCreate, Capability: idpadmin.CapabilityClientsCreate,
			TargetType: "client", RequireTarget: true,
		},
		CommandClientsUpdate: {
			Command: CommandClientsUpdate, Capability: idpadmin.CapabilityClientsUpdate,
			TargetType: "client", RequireTarget: true, RequireExisting: true,
		},
		CommandClientsEnable: {
			Command: CommandClientsEnable, Capability: idpadmin.CapabilityClientsDisable,
			TargetType: "client", RequireTarget: true, RequireExisting: true, RequireReason: true,
		},
		CommandClientsDisable: {
			Command: CommandClientsDisable, Capability: idpadmin.CapabilityClientsDisable,
			TargetType: "client", RequireTarget: true, RequireExisting: true,
			RequireReason: true, ConfirmationText: "DISABLE",
		},
		CommandClientsRotateSecret: {
			Command: CommandClientsRotateSecret, Capability: idpadmin.CapabilityClientSecretRotate,
			TargetType: "client", RequireTarget: true, RequireExisting: true,
			RequireFresh: true, RequireReason: true, ConfirmationText: "ROTATE",
		},
	}
	definition, ok := definitions[command]
	return definition, ok
}
