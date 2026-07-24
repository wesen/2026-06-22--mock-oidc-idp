package idpadminapp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/go-go-golems/tiny-idp/pkg/idp"
	"github.com/go-go-golems/tiny-idp/pkg/idpaccounts"
	"github.com/go-go-golems/tiny-idp/pkg/idpadmin"
	"github.com/go-go-golems/tiny-idp/pkg/idpadminstore"
	"github.com/go-go-golems/tiny-idp/pkg/idpstore"
)

const (
	maxOperatorReason = 500
	maxUserInputBytes = 64 << 10
)

type UserCommandStore interface {
	idpstore.Store
	idpadminstore.Store
}

type UserCommandInput struct {
	Login         string `json:"login,omitempty"`
	Password      string `json:"password,omitempty"`
	Email         string `json:"email,omitempty"`
	EmailVerified *bool  `json:"email_verified,omitempty"`
	DisplayName   string `json:"display_name,omitempty"`
	Locale        string `json:"locale,omitempty"`
	Reason        string `json:"reason,omitempty"`
	Confirmation  string `json:"confirmation,omitempty"`
}

type UserCommandService struct {
	store    UserCommandStore
	executor *Executor
	accounts *idpaccounts.Service
	now      func() time.Time
}

func NewUserCommandService(
	store UserCommandStore,
	executor *Executor,
	accountOptions idpaccounts.Options,
	now func() time.Time,
) (*UserCommandService, error) {
	if store == nil || executor == nil {
		return nil, errors.New("user command store and executor are required")
	}
	if now == nil {
		now = time.Now
	}
	accountOptions.Clock = now
	accountOptions.Audit = idp.NoopSink{}
	accounts, err := idpaccounts.NewService(store, accountOptions)
	if err != nil {
		return nil, err
	}
	return &UserCommandService{store: store, executor: executor, accounts: accounts, now: now}, nil
}

func (s *UserCommandService) Execute(
	ctx context.Context,
	request ExecutionRequest,
	rawInput []byte,
) ([]byte, error) {
	if len(rawInput) == 0 || len(rawInput) > maxUserInputBytes {
		return nil, fmt.Errorf("user command input must contain at most %d bytes", maxUserInputBytes)
	}
	defer clearBytes(rawInput)
	var input UserCommandInput
	decoder := json.NewDecoder(bytes.NewReader(rawInput))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return nil, fmt.Errorf("decode user command input: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, errors.New("user command input contains trailing JSON")
	}
	claims, err := s.executor.Preflight(ctx, request)
	if err != nil {
		return nil, err
	}
	definition, ok := userActionDefinition(claims.Command)
	if !ok || definition.Capability != claims.Capability ||
		definition.TargetType != claims.TargetType ||
		definition.RequireFresh != claims.RequireFresh {
		return nil, ErrUnknownCommand
	}
	reason := strings.TrimSpace(input.Reason)
	if len(reason) > maxOperatorReason {
		return nil, fmt.Errorf("operator reason exceeds %d characters", maxOperatorReason)
	}
	if definition.RequireReason && reason == "" {
		return nil, ErrReasonRequired
	}
	if definition.ConfirmationText != "" && input.Confirmation != definition.ConfirmationText {
		return nil, ErrConfirmationFailed
	}
	request.Reason = reason
	if strings.TrimSpace(request.RequestID) == "" {
		request.RequestID, err = randomID()
		if err != nil {
			return nil, err
		}
	}

	password := []byte(input.Password)
	input.Password = ""
	defer clearBytes(password)

	var preparedCreate idpaccounts.PreparedCreate
	var preparedPassword idpstore.PasswordCredential
	var target idpadmin.UserRow
	switch claims.Command {
	case CommandUsersCreate:
		emailVerified := input.EmailVerified != nil && *input.EmailVerified
		preparedCreate, err = s.accounts.PrepareCreate(ctx, idpaccounts.CreateRequest{
			ID: claims.TargetID, Subject: claims.TargetID, Login: input.Login,
			Password: password, Email: input.Email, EmailVerified: emailVerified,
			Name: input.DisplayName, Locale: input.Locale,
		})
	case CommandUsersUpdate, CommandUsersEnable, CommandUsersDisable,
		CommandUsersUnlock, CommandUsersSetPassword, CommandUsersRevokeAccess:
		target, err = s.store.GetAdminUser(ctx, claims.TargetID)
		if err == nil && claims.Command == CommandUsersSetPassword {
			preparedPassword, err = s.accounts.PreparePassword(
				ctx, target.ID, target.Login, password,
			)
		}
	default:
		err = ErrUnknownCommand
	}
	if err != nil {
		return nil, err
	}

	return s.executor.Execute(ctx, request, func(
		ctx context.Context,
		protocol idpstore.TxStore,
		admin idpadminstore.TxStore,
		claims idpadmin.ActionClaims,
	) ([]byte, error) {
		now := s.now().UTC()
		switch claims.Command {
		case CommandUsersCreate:
			if err := s.accounts.CommitPrepared(ctx, protocol, preparedCreate); err != nil {
				return nil, err
			}
			if err := admin.CreateResourceVersion(ctx, "user", preparedCreate.User.ID, 1, now); err != nil {
				return nil, err
			}
		case CommandUsersUpdate:
			user, err := protocol.GetUser(ctx, target.ID)
			if err != nil {
				return nil, err
			}
			user.Email = strings.TrimSpace(input.Email)
			if input.EmailVerified != nil {
				user.EmailVerified = *input.EmailVerified
			}
			user.Name = strings.TrimSpace(input.DisplayName)
			user.Locale = strings.TrimSpace(input.Locale)
			user.UpdatedAt = now
			if err := user.Validate(); err != nil {
				return nil, err
			}
			if err := protocol.PutUser(ctx, target.Login, user); err != nil {
				return nil, err
			}
		case CommandUsersEnable, CommandUsersDisable:
			user, err := protocol.GetUser(ctx, target.ID)
			if err != nil {
				return nil, err
			}
			user.Disabled = claims.Command == CommandUsersDisable
			user.UpdatedAt = now
			if err := protocol.PutUser(ctx, target.Login, user); err != nil {
				return nil, err
			}
			if user.Disabled {
				if err := revokeUserSecurityArtifacts(ctx, protocol, target.ID, now); err != nil {
					return nil, err
				}
			}
		case CommandUsersUnlock:
			if err := protocol.ResetAccountSecurityState(ctx, target.ID, now); err != nil {
				return nil, err
			}
		case CommandUsersSetPassword:
			if err := protocol.PutPasswordCredential(ctx, preparedPassword); err != nil {
				return nil, err
			}
			if err := protocol.PutAccountSecurityState(ctx, idpstore.AccountSecurityState{UserID: target.ID}); err != nil {
				return nil, err
			}
			if err := revokeUserSecurityArtifacts(ctx, protocol, target.ID, now); err != nil {
				return nil, err
			}
		case CommandUsersRevokeAccess:
			if err := revokeUserSecurityArtifacts(ctx, protocol, target.ID, now); err != nil {
				return nil, err
			}
		default:
			return nil, ErrUnknownCommand
		}
		projection, ok := admin.(idpadminstore.UserProjectionTx)
		if !ok {
			return nil, errors.New("administration store does not support transactional user projections")
		}
		if err := projection.RefreshAdminUserProjection(ctx, claims.TargetID, now); err != nil {
			return nil, err
		}
		reader, ok := admin.(interface {
			GetAdminUser(context.Context, string) (idpadmin.UserRow, error)
		})
		if !ok {
			return nil, errors.New("administration transaction does not support user reads")
		}
		row, err := reader.GetAdminUser(ctx, claims.TargetID)
		if err != nil {
			return nil, err
		}
		return json.Marshal(idpadmin.UserResult{
			User: rowToUserDetail(row), Committed: true, AuditStatus: "pending",
		})
	})
}

func rowToUserDetail(row idpadmin.UserRow) idpadmin.UserDetail {
	return idpadmin.UserDetail{UserRow: row}
}

func revokeUserSecurityArtifacts(
	ctx context.Context,
	protocol idpstore.TxStore,
	userID string,
	now time.Time,
) error {
	security, ok := protocol.(idpstore.UserSecurityTx)
	if !ok {
		return errors.New("protocol store does not support transactional user security revocation")
	}
	return security.RevokeUserSecurityArtifactsTx(ctx, userID, now)
}

func clearBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
